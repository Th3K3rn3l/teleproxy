package goloom

import "encoding/json"

type SignalingMessage struct {
	UID                      string                     `json:"uid,omitempty"`
	Ack                      *Ack                       `json:"ack,omitempty"`
	Hello                    *Hello                     `json:"hello,omitempty"`
	ServerHello              *ServerHello               `json:"serverHello,omitempty"`
	UpdateDescription        *UpdateDescription         `json:"updateDescription,omitempty"`
	VADActivity              *VADActivity               `json:"vadActivity,omitempty"`
	SubscriberSDPOffer       *SubscriberSDPOffer        `json:"subscriberSdpOffer,omitempty"`
	PublisherSDPOffer        *PublisherSDPOffer         `json:"publisherSdpOffer,omitempty"`
	SubscriberSDPAnswer      *SubscriberSDPAnswer       `json:"subscriberSdpAnswer,omitempty"`
	PublisherSDPAnswer       *PublisherSDPAnswer        `json:"publisherSdpAnswer,omitempty"`
	WebRTCIceCandidate       *WebRTCIceCandidateMsg     `json:"webrtcIceCandidate,omitempty"`
	Telemetry                *json.RawMessage           `json:"telemetry,omitempty"`
}

type Ack struct {
	Status Status `json:"status"`
}

type Status struct {
	Code        string `json:"code"`
	Description string `json:"description,omitempty"`
}

type SdkInfo struct {
	Implementation string `json:"implementation"`
	Version        string `json:"version"`
	UserAgent      string `json:"userAgent"`
	HWConcurrency  int    `json:"hwConcurrency"`
}

type Hello struct {
	ParticipantMeta       ParticipantMeta       `json:"participantMeta"`
	ParticipantAttributes ParticipantAttributes `json:"participantAttributes"`
	SendAudio             bool                  `json:"sendAudio"`
	SendVideo             bool                  `json:"sendVideo"`
	SendSharing           bool                  `json:"sendSharing"`
	ParticipantID         string                `json:"participantId"`
	RoomID                string                `json:"roomId"`
	ServiceName           string                `json:"serviceName"`
	Credentials           string                `json:"credentials"`
	CapabilitiesOffer     Capabilities           `json:"capabilitiesOffer"`
	SdkInitializationId   string                `json:"sdkInitializationId"`
	SdkInfo               SdkInfo               `json:"sdkInfo"`
	DisablePublisher      bool                  `json:"disablePublisher"`
	DisableSubscriber     bool                  `json:"disableSubscriber"`
	DisableSubscriberAudio bool                 `json:"disableSubscriberAudio"`
}

type ParticipantMeta struct {
	Name        string `json:"name"`
	Role        string `json:"role"`
	Description string `json:"description"`
	SendAudio   bool   `json:"sendAudio"`
	SendVideo   bool   `json:"sendVideo"`
}

type ParticipantAttributes struct {
	Name        string `json:"name"`
	Role        string `json:"role"`
	Description string `json:"description"`
}

type Capabilities struct {
	OfferAnswerMode              []string `json:"offerAnswerMode"`
	InitialSubscriberOffer       []string `json:"initialSubscriberOffer"`
	SlotsMode                    []string `json:"slotsMode"`
	SimulcastMode                []string `json:"simulcastMode"`
	SelfVadStatus                []string `json:"selfVadStatus"`
	DataChannelSharing           []string `json:"dataChannelSharing"`
	VideoEncoderConfig           []string `json:"videoEncoderConfig"`
	DataChannelVideoCodec        []string `json:"dataChannelVideoCodec"`
	BandwidthLimitationReason    []string `json:"bandwidthLimitationReason"`
	SDKDefaultDeviceManagement   []string `json:"sdkDefaultDeviceManagement"`
	JoinOrderLayout              []string `json:"joinOrderLayout"`
	PinLayout                    []string `json:"pinLayout"`
	SendSelfViewVideoSlot        []string `json:"sendSelfViewVideoSlot"`
	ServerLayoutTransition       []string `json:"serverLayoutTransition"`
	SDKPublisherOptimizeBitrate  []string `json:"sdkPublisherOptimizeBitrate"`
	SDKNetworkLostDetection      []string `json:"sdkNetworkLostDetection"`
	SDKNetworkPathMonitor        []string `json:"sdkNetworkPathMonitor"`
	PublisherVp9                 []string `json:"publisherVp9"`
	SVCMode                      []string `json:"svcMode"`
	SubscriberOfferAsyncAck      []string `json:"subscriberOfferAsyncAck"`
	AndroidBluetoothRoutingFix   []string `json:"androidBluetoothRoutingFix"`
	FixedIceCandidatesPoolSize   []string `json:"fixedIceCandidatesPoolSize"`
	SDKAndroidTelecomIntegration []string `json:"sdkAndroidTelecomIntegration"`
	SetActiveCodecsMode          []string `json:"setActiveCodecsMode"`
	SubscriberDtlsPassiveMode    []string `json:"subscriberDtlsPassiveMode"`
	PublisherOpusDred            []string `json:"publisherOpusDred"`
	PublisherOpusLowBitrate      []string `json:"publisherOpusLowBitrate"`
	SDKAndroidDestroySessionOnTaskRemoved []string `json:"sdkAndroidDestroySessionOnTaskRemoved"`
	PublisherOpusDredAndroid     []string `json:"publisherOpusDredAndroid"`
	PublisherOpusDredIos         []string `json:"publisherOpusDredIos"`
	SubscriberOpusDredAndroid    []string `json:"subscriberOpusDredAndroid"`
	SubscriberOpusDredIos        []string `json:"subscriberOpusDredIos"`
	SVCModes                     []string `json:"svcModes"`
	ReportTelemetryModes         []string `json:"reportTelemetryModes"`
	KeepDefaultDevicesModes      []string `json:"keepDefaultDevicesModes"`
}

type ServerHello struct {
	CapabilitiesAnswer CapabilitiesAnswer `json:"capabilitiesAnswer"`
	ServingComponents  []ServingComponent `json:"servingComponents"`
	CurrentTime        *int64             `json:"currentTime,omitempty"`
}

type CapabilitiesAnswer struct {
	OfferAnswerMode            string `json:"offerAnswerMode"`
	InitialSubscriberOffer     string `json:"initialSubscriberOffer"`
	SlotsMode                  string `json:"slotsMode"`
	SimulcastMode              string `json:"simulcastMode"`
	SelfVadStatus              string `json:"selfVadStatus"`
	DataChannelSharing         string `json:"dataChannelSharing"`
	VideoEncoderConfig         string `json:"videoEncoderConfig"`
	DataChannelVideoCodec      string `json:"dataChannelVideoCodec"`
	BandwidthLimitationReason  string `json:"bandwidthLimitationReason"`
	ServerLayoutTransition     string `json:"serverLayoutTransition"`
	PinLayout                  string `json:"pinLayout"`
	JoinOrderLayout            string `json:"joinOrderLayout"`
	SendSelfViewVideoSlot      string `json:"sendSelfViewVideoSlot"`
	SDKDefaultDeviceManagement string `json:"sdkDefaultDeviceManagement"`
	SDKPublisherOptimizeBitrate string `json:"sdkPublisherOptimizeBitrate"`
	SDKNetworkPathMonitor      string `json:"sdkNetworkPathMonitor"`
	PublisherVp9               string `json:"publisherVp9"`
	SVCMode                    string `json:"svcMode"`
	SDKNetworkLostDetection    string `json:"sdkNetworkLostDetection"`
	FixedIceCandidatesPoolSize string `json:"fixedIceCandidatesPoolSize"`
	SubscriberOfferAsyncAck    string `json:"subscriberOfferAsyncAck"`
	SubscriberDtlsPassiveMode  string `json:"subscriberDtlsPassiveMode"`
}

type ServingComponent struct {
	Type string `json:"type"`
	Host string `json:"host,omitempty"`
}

type UpdateDescription struct {
	Description []ParticipantDescription `json:"description"`
}

type ParticipantDescription struct {
	ID                      string              `json:"id"`
	Meta                    ParticipantMeta     `json:"meta"`
	ParticipantAttributes   map[string]string   `json:"participantAttributes"`
	SendAudio               bool               `json:"sendAudio"`
	SendVideo               bool               `json:"sendVideo"`
	SendSharing             bool               `json:"sendSharing"`
	HideFromParticipantsList bool              `json:"hideFromParticipantsList"`
	NetworkScore            string             `json:"networkScore"`
	ConnectionType          string             `json:"connectionType"`
}

type VADActivity struct {
	Active bool `json:"active"`
}

type SubscriberSDPOffer struct {
	PCSeq  int    `json:"pcSeq"`
	SDP    string `json:"sdp"`
}

type PublisherSDPOffer struct {
	PCSeq  int    `json:"pcSeq"`
	SDP    string `json:"sdp"`
}

type SubscriberSDPAnswer struct {
	SDP string `json:"sdp"`
}

type PublisherSDPAnswer struct {
	PCSeq int    `json:"pcSeq"`
	SDP   string `json:"sdp"`
}

type WebRTCIceCandidateMsg struct {
	PCSeq            int    `json:"pcSeq,omitempty"`
	Candidate        string `json:"candidate"`
	SDPMid           string `json:"sdpMid"`
	SDPLineIndex     int    `json:"sdpMlineIndex,omitempty"`
	Target           string `json:"target,omitempty"`
	UsernameFragment string `json:"usernameFragment,omitempty"`
}
