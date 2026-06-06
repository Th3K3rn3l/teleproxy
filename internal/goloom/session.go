package goloom

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

type SessionConfig struct {
	DisplayName   string
	ConferenceURI string
	UID           string
	SessionCookie string
}

type joinInfo struct {
	roomID         string
	peerID         string
	mediaServerURL string
	serviceName    string
	credentials    string
}

type Session struct {
	config    SessionConfig
	api       *Client
	signaling *Signaling
	peerConn  *webrtc.PeerConnection
	dataChan  *webrtc.DataChannel

	ji *joinInfo

	mu               sync.Mutex
	dataChannelReady chan struct{}
	onData           func([]byte)
	onError          func(error)
	onClose          func()
}

func NewSession(config SessionConfig) *Session {
	s := &Session{
		config:           config,
		dataChannelReady: make(chan struct{}),
	}
	s.api = NewClient(config.UID, config.SessionCookie)
	return s
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

func (s *Session) Start(ctx context.Context) error {
	roomID := extractRoomID(s.config.ConferenceURI)
	if roomID == "" {
		return fmt.Errorf("invalid conference URL: %s", s.config.ConferenceURI)
	}

	ji := &joinInfo{
		roomID:         roomID,
		peerID:         uuid.New().String(),
		mediaServerURL: "wss://goloom.strm.yandex.net/join",
		serviceName:    "telemost",
		credentials:    "",
	}

	conf, err := s.api.JoinConference(s.config.ConferenceURI)
	if err != nil {
		log.Printf("REST API join failed (proceeding without credentials): %v", err)
	} else {
		log.Printf("REST API join OK: room=%s peer=%s hasCredentials=%v ws=%s",
			conf.RoomID, conf.PeerID, conf.Credentials != "", conf.ClientConfiguration.MediaServerURL)
		ji.roomID = conf.RoomID
		ji.peerID = conf.PeerID
		if conf.ClientConfiguration.MediaServerURL != "" {
			ji.mediaServerURL = conf.ClientConfiguration.MediaServerURL
		}
		if conf.ClientConfiguration.ServiceName != "" {
			ji.serviceName = conf.ClientConfiguration.ServiceName
		}
		if conf.Credentials != "" {
			ji.credentials = conf.Credentials
		}
	}

	log.Printf("Connecting to WS with: room=%s peer=%s creds_len=%d", ji.roomID, ji.peerID, len(ji.credentials))

	s.ji = ji

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

	if err := s.signaling.Connect(ctx, ji.mediaServerURL); err != nil {
		return fmt.Errorf("signaling connect: %w", err)
	}

	if err := s.signaling.SendHello(
		ji.roomID,
		ji.peerID,
		ji.serviceName,
		ji.credentials,
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
			log.Printf("ICE: gathering complete (end-of-candidates)")
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
		log.Printf("ICE: sending candidate target=PUBLISHER candidate=%s", cJSON.Candidate)
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
		log.Printf("SDP: no peer connection")
		return
	}

	// 1. Set remote description (SFU's subscriber offer)
	log.Printf("SDP: setting remote description (subscriber offer)")
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  msg.SDP,
	}
	if err := p.SetRemoteDescription(offer); err != nil {
		s.handleError(fmt.Errorf("set remote desc: %w", err))
		return
	}

	// 2. Create answer (subscriber side)
	log.Printf("SDP: creating subscriber answer")
	answer, err := p.CreateAnswer(nil)
	if err != nil {
		s.handleError(fmt.Errorf("create answer: %w", err))
		return
	}

	// 3. Set local description with answer → stable state
	log.Printf("SDP: setting local description (answer)")
	if err := p.SetLocalDescription(answer); err != nil {
		s.handleError(fmt.Errorf("set local desc: %w", err))
		return
	}

	// 4. SAVE subscriber answer SDP with ICE credentials from SetLocalDescription
	// Match browser: use passive DTLS role (SFU is active) and limit max-message-size
	subscriberAnswerSDP := fixAnswerSDP(p.LocalDescription().SDP)

	// 5. Create publisher offer (fresh negotiation from stable)
	log.Printf("SDP: creating publisher offer")
	pubOffer, err := p.CreateOffer(nil)
	if err != nil {
		s.handleError(fmt.Errorf("create publisher offer: %w", err))
		return
	}

	// 6. Set local description with publisher offer → have-local-offer (triggers ICE gathering)
	if err := p.SetLocalDescription(pubOffer); err != nil {
		s.handleError(fmt.Errorf("set publisher offer: %w", err))
		return
	}

	// 7. Send both SDPs IMMEDIATELY (don't wait for ICE gather - browser does trickle ICE)
	//    ICE candidates will arrive via OnICECandidate after SDPs are sent
	log.Printf("SDP: sending publisher offer (pcSeq=1, sdp_len=%d)", len(p.LocalDescription().SDP))
	log.Printf("PUBLISHER_OFFER_SDP: %s", p.LocalDescription().SDP)
	s.signaling.SendPublisherSDPOffer(p.LocalDescription().SDP, 1)

	log.Printf("SDP: sending subscriber answer with pcSeq=%d (sdp_len=%d)", msg.PCSeq, len(subscriberAnswerSDP))
	log.Printf("SUBSCRIBER_ANSWER_SDP: %s", subscriberAnswerSDP)
	s.signaling.SendSubscriberSDPAnswer(subscriberAnswerSDP, msg.PCSeq)
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
	defer s.mu.Unlock()
}

func (s *Session) OnDisconnected(err error) {
	log.Printf("WebSocket disconnected: %v", err)
	if s.onClose != nil {
		s.onClose()
	}
}

func (s *Session) HandleAck(uid string, status Status) {}

func (s *Session) OnParticipantUpdate(desc []ParticipantDescription) {}

func (s *Session) handleError(err error) {
	if s.onError != nil {
		s.onError(err)
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func fixAnswerSDP(sdp string) string {
	sdp = strings.ReplaceAll(sdp, "\na=setup:active", "\na=setup:passive")
	sdp = strings.ReplaceAll(sdp, "\na=ice-options:trickle\n", "\n")
	sdp = strings.ReplaceAll(sdp, "a=max-message-size:1073741823", "a=max-message-size:262144")
	return sdp
}

func extractRoomID(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.TrimRight(u.Path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
