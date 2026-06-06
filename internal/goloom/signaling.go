package goloom

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
	dialer.Subprotocols = []string{}

	header := http.Header{}
	header.Set("Origin", "https://telemost.yandex.ru")

	conn, _, err := dialer.DialContext(ctx, urlStr, header)
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
				Name:        displayName,
				Role:        "SPEAKER",
				Description: "",
				SendAudio:   false,
				SendVideo:   false,
			},
			ParticipantAttributes: ParticipantAttributes{
				Name:        displayName,
				Role:        "SPEAKER",
				Description: "",
			},
			SendAudio:      true,
			SendVideo:      false,
			SendSharing:    false,
			ParticipantID:  participantID,
			RoomID:         roomID,
			ServiceName:    serviceName,
			Credentials:    credentials,
			SdkInitializationId: newUUID(),
			SdkInfo: SdkInfo{
				Implementation: "browser",
				Version:        "5.29.0",
				UserAgent:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36",
				HWConcurrency:  12,
			},
			DisablePublisher:      false,
			DisableSubscriber:     false,
			DisableSubscriberAudio: false,
			CapabilitiesOffer: Capabilities{
				OfferAnswerMode:              []string{"SEPARATE"},
				InitialSubscriberOffer:       []string{"ON_HELLO"},
				SlotsMode:                    []string{"FROM_CONTROLLER"},
				SimulcastMode:                []string{"DISABLED", "STATIC"},
				SelfVadStatus:                []string{"FROM_SERVER", "FROM_CLIENT"},
				DataChannelSharing:           []string{"TO_RTP"},
				VideoEncoderConfig:           []string{"NO_CONFIG", "ONLY_INIT_CONFIG", "RUNTIME_CONFIG"},
				DataChannelVideoCodec:        []string{"VP8", "UNIQUE_CODEC_FROM_TRACK_DESCRIPTION"},
				BandwidthLimitationReason:    []string{"BANDWIDTH_REASON_DISABLED", "BANDWIDTH_REASON_ENABLED"},
				SDKDefaultDeviceManagement:   []string{"SDK_DEFAULT_DEVICE_MANAGEMENT_DISABLED", "SDK_DEFAULT_DEVICE_MANAGEMENT_ENABLED"},
				JoinOrderLayout:              []string{"JOIN_ORDER_LAYOUT_DISABLED", "JOIN_ORDER_LAYOUT_ENABLED"},
				PinLayout:                    []string{"PIN_LAYOUT_DISABLED"},
				SendSelfViewVideoSlot:        []string{"SEND_SELF_VIEW_VIDEO_SLOT_DISABLED", "SEND_SELF_VIEW_VIDEO_SLOT_ENABLED"},
				ServerLayoutTransition:       []string{"SERVER_LAYOUT_TRANSITION_DISABLED"},
				SDKPublisherOptimizeBitrate:  []string{"SDK_PUBLISHER_OPTIMIZE_BITRATE_DISABLED", "SDK_PUBLISHER_OPTIMIZE_BITRATE_FULL", "SDK_PUBLISHER_OPTIMIZE_BITRATE_ONLY_SELF"},
				SDKNetworkLostDetection:      []string{"SDK_NETWORK_LOST_DETECTION_DISABLED"},
				SDKNetworkPathMonitor:        []string{"SDK_NETWORK_PATH_MONITOR_DISABLED"},
				PublisherVp9:                 []string{"PUBLISH_VP9_DISABLED", "PUBLISH_VP9_ENABLED"},
				SVCMode:                      []string{"SVC_MODE_DISABLED", "SVC_MODE_L3T3", "SVC_MODE_L3T3_KEY"},
				SubscriberOfferAsyncAck:      []string{"SUBSCRIBER_OFFER_ASYNC_ACK_DISABLED", "SUBSCRIBER_OFFER_ASYNC_ACK_ENABLED"},
				AndroidBluetoothRoutingFix:   []string{"ANDROID_BLUETOOTH_ROUTING_FIX_DISABLED"},
				FixedIceCandidatesPoolSize:   []string{"FIXED_ICE_CANDIDATES_POOL_SIZE_DISABLED"},
				SDKAndroidTelecomIntegration: []string{"SDK_ANDROID_TELECOM_INTEGRATION_DISABLED"},
				SetActiveCodecsMode:          []string{"SET_ACTIVE_CODECS_MODE_DISABLED", "SET_ACTIVE_CODECS_MODE_VIDEO_ONLY"},
				SubscriberDtlsPassiveMode:    []string{"SUBSCRIBER_DTLS_PASSIVE_MODE_DISABLED", "SUBSCRIBER_DTLS_PASSIVE_MODE_ENABLED"},
				PublisherOpusDred:            []string{"PUBLISHER_OPUS_DRED_DISABLED"},
				PublisherOpusLowBitrate:      []string{"PUBLISHER_OPUS_LOW_BITRATE_DISABLED"},
				SDKAndroidDestroySessionOnTaskRemoved: []string{"SDK_ANDROID_DESTROY_SESSION_ON_TASK_REMOVED_DISABLED"},
				PublisherOpusDredAndroid:     []string{"PUBLISHER_OPUS_DRED_ANDROID_DISABLED"},
				PublisherOpusDredIos:         []string{"PUBLISHER_OPUS_DRED_IOS_DISABLED"},
				SubscriberOpusDredAndroid:    []string{"SUBSCRIBER_OPUS_DRED_ANDROID_DISABLED"},
				SubscriberOpusDredIos:        []string{"SUBSCRIBER_OPUS_DRED_IOS_DISABLED"},
				SVCModes:                     []string{"FALSE"},
				ReportTelemetryModes:         []string{"TRUE"},
				KeepDefaultDevicesModes:      []string{"FALSE"},
			},
		},
	}

	ch, err := s.sendMessage(hello)
	if err != nil {
		return err
	}
	log.Printf("WS hello sent: room=%s participant=%s creds_len=%d, waiting for ack...", roomID, participantID, len(credentials))
	select {
	case status := <-ch:
		if status.Code != "OK" {
			return fmt.Errorf("hello rejected: %s", status.Description)
		}
		log.Printf("WS hello ack OK")
	case <-time.After(15 * time.Second):
		return fmt.Errorf("hello ack timeout")
	}
	return nil
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
	go func() {
		select {
		case status := <-ch:
			if status.Code != "OK" {
				log.Printf("publisher offer rejected: %s", status.Description)
			}
		case <-time.After(15 * time.Second):
			log.Printf("publisher offer ack timeout")
		}
	}()
	return nil
}

func (s *Signaling) SendSubscriberSDPAnswer(sdp string, pcSeq int) error {
	msg := SignalingMessage{
		UID: newUUID(),
		SubscriberSDPAnswer: &SubscriberSDPAnswer{
			PCSeq: pcSeq,
			SDP:   sdp,
		},
	}
	ch := s.registerPending(msg.UID)
	if err := s.sendJSON(msg); err != nil {
		return err
	}
	go func() {
		select {
		case status := <-ch:
			if status.Code != "OK" {
				log.Printf("subscriber answer rejected: %s", status.Description)
			}
		case <-time.After(15 * time.Second):
			log.Printf("subscriber answer ack timeout")
		}
	}()
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
	log.Printf("WS recv: type=ack=%v serverHello=%v subOffer=%v pubAnswer=%v ice=%v update=%v vad=%v",
		msg.Ack != nil, msg.ServerHello != nil, msg.SubscriberSDPOffer != nil,
		msg.PublisherSDPAnswer != nil, msg.WebRTCIceCandidate != nil,
		msg.UpdateDescription != nil, msg.VADActivity != nil)

	if msg.Ack != nil {
		log.Printf("WS ack: uid=%s code=%s desc=%s", msg.UID[:8], msg.Ack.Status.Code, msg.Ack.Status.Description)
		s.resolvePending(msg.UID, msg.Ack.Status)
		s.handlers.HandleAck(msg.UID, msg.Ack.Status)
		return
	}

	if msg.ServerHello != nil {
		log.Printf("WS serverHello received")
		s.mu.Lock()
		s.state = StateInRoom
		s.mu.Unlock()
		s.SendAck(msg.UID)
		s.handlers.OnConnected(*msg.ServerHello)
		return
	}

	if msg.SubscriberSDPOffer != nil {
		log.Printf("WS subscriberSdpOffer received (sdp len=%d)", len(msg.SubscriberSDPOffer.SDP))
		log.Printf("SUBSCRIBER_OFFER_SDP: %s", msg.SubscriberSDPOffer.SDP)
		s.handlers.OnSubscriberSDPOffer(*msg.SubscriberSDPOffer)
		return
	}

	if msg.PublisherSDPAnswer != nil {
		log.Printf("WS publisherSdpAnswer received (sdp len=%d)", len(msg.PublisherSDPAnswer.SDP))
		log.Printf("PUBLISHER_ANSWER_SDP: %s", msg.PublisherSDPAnswer.SDP)
		s.SendAck(msg.UID)
		s.handlers.OnPublisherSDPAnswer(*msg.PublisherSDPAnswer)
		return
	}

	if msg.WebRTCIceCandidate != nil {
		log.Printf("WS ice candidate: target=%s", msg.WebRTCIceCandidate.Target)
		s.handlers.OnIceCandidate(
			msg.WebRTCIceCandidate.Target,
			msg.WebRTCIceCandidate.Candidate,
			msg.WebRTCIceCandidate.SDPMid,
			msg.WebRTCIceCandidate.SDPLineIndex,
		)
		return
	}

	if msg.UpdateDescription != nil {
		log.Printf("WS updateDescription: %d participants", len(msg.UpdateDescription.Description))
		s.handlers.OnParticipantUpdate(msg.UpdateDescription.Description)
		s.SendAck(msg.UID)
		return
	}

	if msg.VADActivity != nil {
		log.Printf("WS vadActivity")
		s.SendAck(msg.UID)
		return
	}
}
