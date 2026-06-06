package goloom

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type SignalingState int

const (
	StateDisconnected SignalingState = iota
	StateConnecting
	StateConnected
	StateInRoom
)

type SignalingHandler interface {
	OnSubscriberSDPOffer(msg SubscriberSDPOffer)
	OnPublisherSDPAnswer(msg PublisherSDPAnswer)
	OnIceCandidate(target string, candidate string, sdpMid string, sdpMLineIndex int)
	OnParticipantUpdate(desc []ParticipantDescription)
	OnConnected(serverHello ServerHello)
	OnDisconnected(err error)
	HandleAck(uid string, status Status)
}

type Signaling struct {
	conn     *websocket.Conn
	mu       sync.Mutex
	state    SignalingState
	handlers SignalingHandler
	pending  map[string]chan Status
	pendingMu sync.Mutex
	done     chan struct{}

	roomID       string
	participantID string
	serviceName  string
	credentials  string
	displayName  string
}

func NewSignaling(handler SignalingHandler) *Signaling {
	return &Signaling{
		handlers: handler,
		pending:  make(map[string]chan Status),
		done:     make(chan struct{}),
	}
}

func (s *Signaling) Connect(ctx context.Context, urlStr string) error {
	s.mu.Lock()
	s.state = StateConnecting
	s.mu.Unlock()

	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	conn, _, err := dialer.DialContext(ctx, urlStr, nil)
	if err != nil {
		s.mu.Lock()
		s.state = StateDisconnected
		s.mu.Unlock()
		return fmt.Errorf("dial websocket: %w", err)
	}

	s.mu.Lock()
	s.conn = conn
	s.state = StateConnected
	s.mu.Unlock()

	go s.readLoop()

	return nil
}

func (s *Signaling) SendHello(roomID, participantID, serviceName, credentials, displayName string) error {
	s.roomID = roomID
	s.participantID = participantID
	s.serviceName = serviceName
	s.credentials = credentials
	s.displayName = displayName

	hello := SignalingMessage{
		UID: newUUID(),
		Hello: &Hello{
			ParticipantMeta: ParticipantMeta{
				Name:      displayName,
				Role:      "SPEAKER",
				SendAudio: false,
				SendVideo: false,
			},
			ParticipantAttributes: ParticipantAttributes{
				Name:      displayName,
				Role:      "SPEAKER",
				Description: "",
			},
			SendAudio:     false,
			SendVideo:     false,
			SendSharing:   false,
			ParticipantID: participantID,
			RoomID:        roomID,
			ServiceName:   serviceName,
			Credentials:   credentials,
			CapabilitiesOffer: Capabilities{
				OfferAnswerMode:            []string{"SEPARATE"},
				InitialSubscriberOffer:     []string{"ON_HELLO"},
				SlotsMode:                  []string{"FROM_CONTROLLER"},
				SimulcastMode:              []string{"DISABLED"},
				SelfVadStatus:              []string{"FROM_SERVER"},
				DataChannelSharing:         []string{"TO_RTP"},
				VideoEncoderConfig:         []string{"NO_CONFIG"},
				DataChannelVideoCodec:      []string{"VP8"},
				BandwidthLimitationReason:  []string{"BANDWIDTH_REASON_DISABLED"},
				SDKDefaultDeviceManagement: []string{"SDK_DEFAULT_DEVICE_MANAGEMENT_DISABLED"},
				JoinOrderLayout:            []string{"JOIN_ORDER_LAYOUT_ENABLED"},
				PinLayout:                  []string{"PIN_LAYOUT_DISABLED"},
				SendSelfViewVideoSlot:      []string{"SEND_SELF_VIEW_VIDEO_SLOT_DISABLED"},
				ServerLayoutTransition:     []string{"SERVER_LAYOUT_TRANSITION_DISABLED"},
				SDKPublisherOptimizeBitrate: []string{"SDK_PUBLISHER_OPTIMIZE_BITRATE_DISABLED"},
				SDKNetworkLostDetection:    []string{"SDK_NETWORK_LOST_DETECTION_DISABLED"},
				SDKNetworkPathMonitor:      []string{"SDK_NETWORK_PATH_MONITOR_DISABLED"},
				PublisherVp9:               []string{"PUBLISH_VP9_DISABLED"},
				SVCMode:                    []string{"SVC_MODE_DISABLED"},
				SubscriberOfferAsyncAck:    []string{"SUBSCRIBER_OFFER_ASYNC_ACK_DISABLED"},
			},
		},
	}

	_, err := s.sendMessage(hello)
	return err
}

func (s *Signaling) SendPublisherSDPOffer(sdp string, pcSeq int) error {
	msg := SignalingMessage{
		UID: newUUID(),
		PublisherSDPOffer: &PublisherSDPOffer{
			PCSeq: pcSeq,
			SDP:   sdp,
		},
	}
	ch := s.registerPending(msg.UID)
	if err := s.sendJSON(msg); err != nil {
		return err
	}
	select {
	case status := <-ch:
		if status.Code != "OK" {
			return fmt.Errorf("publisher offer rejected: %s", status.Description)
		}
	case <-time.After(15 * time.Second):
		return fmt.Errorf("publisher offer ack timeout")
	}
	return nil
}

func (s *Signaling) SendSubscriberSDPAnswer(sdp string) error {
	msg := SignalingMessage{
		UID: newUUID(),
		SubscriberSDPAnswer: &SubscriberSDPAnswer{
			SDP: sdp,
		},
	}
	ch := s.registerPending(msg.UID)
	if err := s.sendJSON(msg); err != nil {
		return err
	}
	select {
	case status := <-ch:
		if status.Code != "OK" {
			return fmt.Errorf("subscriber answer rejected: %s", status.Description)
		}
	case <-time.After(15 * time.Second):
		return fmt.Errorf("subscriber answer ack timeout")
	}
	return nil
}

func (s *Signaling) SendICECandidate(target string, candidate string, sdpMid string, sdpMLineIndex int) error {
	msg := SignalingMessage{
		UID: newUUID(),
		WebRTCIceCandidate: &WebRTCIceCandidateMsg{
			Candidate:    candidate,
			SDPMid:       sdpMid,
			SDPLineIndex: sdpMLineIndex,
			Target:       target,
		},
	}
	return s.sendJSON(msg)
}

func (s *Signaling) SendAck(uid string) error {
	msg := SignalingMessage{
		UID: uid,
		Ack: &Ack{
			Status: Status{Code: "OK"},
		},
	}
	return s.sendJSON(msg)
}

func (s *Signaling) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.done:
		return nil
	default:
		close(s.done)
	}
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

func (s *Signaling) sendJSON(msg any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return fmt.Errorf("not connected")
	}
	return s.conn.WriteJSON(msg)
}

func (s *Signaling) sendMessage(msg SignalingMessage) (chan Status, error) {
	ch := s.registerPending(msg.UID)
	if err := s.sendJSON(msg); err != nil {
		s.unregisterPending(msg.UID)
		return nil, err
	}
	return ch, nil
}

func (s *Signaling) registerPending(uid string) chan Status {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	ch := make(chan Status, 1)
	s.pending[uid] = ch
	return ch
}

func (s *Signaling) unregisterPending(uid string) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	delete(s.pending, uid)
}

func (s *Signaling) resolvePending(uid string, status Status) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	if ch, ok := s.pending[uid]; ok {
		ch <- status
		close(ch)
		delete(s.pending, uid)
	}
}

func (s *Signaling) readLoop() {
	for {
		s.mu.Lock()
		conn := s.conn
		s.mu.Unlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			s.mu.Lock()
			s.state = StateDisconnected
			s.mu.Unlock()
			s.handlers.OnDisconnected(fmt.Errorf("read error: %w", err))
			return
		}

		var msg SignalingMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		s.dispatch(msg)
	}
}

func (s *Signaling) dispatch(msg SignalingMessage) {
	if msg.Ack != nil {
		s.resolvePending(msg.UID, msg.Ack.Status)
		s.handlers.HandleAck(msg.UID, msg.Ack.Status)
		return
	}

	if msg.ServerHello != nil {
		s.mu.Lock()
		s.state = StateInRoom
		s.mu.Unlock()
		s.handlers.OnConnected(*msg.ServerHello)
		return
	}

	if msg.SubscriberSDPOffer != nil {
		s.handlers.OnSubscriberSDPOffer(*msg.SubscriberSDPOffer)
		return
	}

	if msg.PublisherSDPAnswer != nil {
		s.handlers.OnPublisherSDPAnswer(*msg.PublisherSDPAnswer)
		return
	}

	if msg.WebRTCIceCandidate != nil {
		s.handlers.OnIceCandidate(
			msg.WebRTCIceCandidate.Target,
			msg.WebRTCIceCandidate.Candidate,
			msg.WebRTCIceCandidate.SDPMid,
			msg.WebRTCIceCandidate.SDPLineIndex,
		)
		return
	}

	if msg.UpdateDescription != nil {
		s.handlers.OnParticipantUpdate(msg.UpdateDescription.Description)
		// Ack automatically
		s.SendAck(msg.UID)
		return
	}

	if msg.VADActivity != nil {
		s.SendAck(msg.UID)
		return
	}
}
