package goloom

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
)

type SessionMode int

const (
	ModeCreate SessionMode = iota
	ModeJoin
)

type SessionConfig struct {
	Mode           SessionMode
	UID            string
	SessionCookie  string
	DisplayName    string
	ConferenceURI  string
}

type Session struct {
	config    SessionConfig
	api       *Client
	signaling *Signaling
	peerConn  *webrtc.PeerConnection
	dataChan  *webrtc.DataChannel

	conf       *CreateConferenceResponse
	serverHello *ServerHello

	mu               sync.Mutex
	dataChannelReady chan struct{}
	negotiationDone  chan struct{}
	onData           func([]byte)
	onError          func(error)
	onClose          func()
}

func NewSession(config SessionConfig) *Session {
	return &Session{
		config:           config,
		api:              NewClient(config.UID, config.SessionCookie),
		dataChannelReady: make(chan struct{}),
		negotiationDone:  make(chan struct{}),
	}
}

func (s *Session) OnData(f func([]byte)) {
	s.onData = f
}

func (s *Session) OnError(f func(error)) {
	s.onError = f
}

func (s *Session) OnClose(f func()) {
	s.onClose = f
}

func (s *Session) ConferenceURL() string {
	if s.conf != nil {
		return s.conf.URI
	}
	return ""
}

func (s *Session) Start(ctx context.Context) error {
	var err error
	switch s.config.Mode {
	case ModeCreate:
		s.conf, err = s.api.CreateConference()
	case ModeJoin:
		s.conf, err = s.api.JoinConference(s.config.ConferenceURI)
	}
	if err != nil {
		return fmt.Errorf("conference setup: %w", err)
	}

	pc, dc, err := s.createPeerConnection()
	if err != nil {
		return fmt.Errorf("create pc: %w", err)
	}

	s.peerConn = pc
	s.dataChan = dc

	dc.OnOpen(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		select {
		case <-s.dataChannelReady:
		default:
			close(s.dataChannelReady)
		}
	})

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if s.onData != nil {
			s.onData(msg.Data)
		}
	})

	s.signaling = NewSignaling(s)

	if err := s.signaling.Connect(ctx, s.conf.ClientConfiguration.MediaServerURL); err != nil {
		return fmt.Errorf("signaling connect: %w", err)
	}

	if err := s.signaling.SendHello(
		s.conf.RoomID,
		s.conf.PeerID,
		s.conf.ClientConfiguration.ServiceName,
		s.conf.Credentials,
		s.config.DisplayName,
	); err != nil {
		return fmt.Errorf("hello: %w", err)
	}

	return nil
}

func (s *Session) Send(data []byte) error {
	s.mu.Lock()
	dc := s.dataChan
	s.mu.Unlock()
	if dc == nil {
		return fmt.Errorf("data channel not ready")
	}
	return dc.Send(data)
}

func (s *Session) WaitForDataChannel(ctx context.Context) error {
	select {
	case <-s.dataChannelReady:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for data channel")
	}
}

func (s *Session) Close() {
	if s.signaling != nil {
		s.signaling.Close()
	}
	if s.peerConn != nil {
		s.peerConn.Close()
	}
}

func (s *Session) createPeerConnection() (*webrtc.PeerConnection, *webrtc.DataChannel, error) {
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		return nil, nil, fmt.Errorf("register codecs: %w", err)
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))
	pc, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.rtc.yandex.net:3478"}},
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("new pc: %w", err)
	}

	dc, err := pc.CreateDataChannel("proxy", &webrtc.DataChannelInit{
		Ordered: boolPtr(true),
	})
	if err != nil {
		pc.Close()
		return nil, nil, fmt.Errorf("create dc: %w", err)
	}

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		cJSON := c.ToJSON()
		if cJSON.Candidate == "" {
			return
		}
		sdpMid := ""
		if cJSON.SDPMid != nil {
			sdpMid = *cJSON.SDPMid
		}
		sdpMLineIndex := 0
		if cJSON.SDPMLineIndex != nil {
			sdpMLineIndex = int(*cJSON.SDPMLineIndex)
		}
		s.signaling.SendICECandidate("PUBLISHER", cJSON.Candidate, sdpMid, sdpMLineIndex)
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateDisconnected ||
			state == webrtc.PeerConnectionStateFailed ||
			state == webrtc.PeerConnectionStateClosed {
			if s.onClose != nil {
				s.onClose()
			}
		}
	})

	return pc, dc, nil
}

// SignalingHandler implementation

func (s *Session) OnSubscriberSDPOffer(msg SubscriberSDPOffer) {
	p := s.peerConn
	if p == nil {
		return
	}

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  msg.SDP,
	}

	if err := p.SetRemoteDescription(offer); err != nil {
		s.handleError(fmt.Errorf("set remote desc: %w", err))
		return
	}

	answer, err := p.CreateAnswer(nil)
	if err != nil {
		s.handleError(fmt.Errorf("create answer: %w", err))
		return
	}

	gatherComplete := webrtc.GatheringCompletePromise(p)
	if err := p.SetLocalDescription(answer); err != nil {
		s.handleError(fmt.Errorf("set local desc: %w", err))
		return
	}
	<-gatherComplete

	if err := s.signaling.SendSubscriberSDPAnswer(p.LocalDescription().SDP); err != nil {
		s.handleError(fmt.Errorf("send subscriber answer: %w", err))
		return
	}

	dc := s.dataChan
	if dc == nil || dc.ReadyState() != webrtc.DataChannelStateOpen {
		go s.sendPublisherOffer()
	}
}

func (s *Session) sendPublisherOffer() {
	time.Sleep(100 * time.Millisecond)

	p := s.peerConn
	if p == nil {
		return
	}

	offer, err := p.CreateOffer(nil)
	if err != nil {
		s.handleError(fmt.Errorf("create publisher offer: %w", err))
		return
	}

	var gatherComplete <-chan struct{}
	if offer.Type == webrtc.SDPTypeOffer {
		gatherComplete = webrtc.GatheringCompletePromise(p)
	}

	if err := p.SetLocalDescription(offer); err != nil {
		s.handleError(fmt.Errorf("set publisher offer: %w", err))
		return
	}

	if gatherComplete != nil {
		<-gatherComplete
	}

	if err := s.signaling.SendPublisherSDPOffer(p.LocalDescription().SDP, 1); err != nil {
		s.handleError(fmt.Errorf("send publisher offer: %w", err))
	}
}

func (s *Session) OnPublisherSDPAnswer(msg PublisherSDPAnswer) {
	p := s.peerConn
	if p == nil {
		return
	}

	desc := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  msg.SDP,
	}

	if err := p.SetRemoteDescription(desc); err != nil {
		s.handleError(fmt.Errorf("set publisher remote desc: %w", err))
	}
}

func (s *Session) OnIceCandidate(target string, candidate string, sdpMid string, sdpMLineIndex int) {
	if s.peerConn == nil {
		return
	}
	c := webrtc.ICECandidateInit{Candidate: candidate}
	if err := s.peerConn.AddICECandidate(c); err != nil {
		s.handleError(fmt.Errorf("add ice candidate: %w", err))
	}
}

func (s *Session) OnConnected(serverHello ServerHello) {
	s.mu.Lock()
	s.serverHello = &serverHello
	s.mu.Unlock()
}

func (s *Session) OnDisconnected(err error) {
	if s.onClose != nil {
		s.onClose()
	}
}

func (s *Session) HandleAck(uid string, status Status) {}

func (s *Session) OnParticipantUpdate(desc []ParticipantDescription) {
	s.SendAckFromSession(desc)
}

func (s *Session) SendAckFromSession(desc []ParticipantDescription) {}

func (s *Session) handleError(err error) {
	if s.onError != nil {
		s.onError(err)
	}
}

func boolPtr(b bool) *bool {
	return &b
}
