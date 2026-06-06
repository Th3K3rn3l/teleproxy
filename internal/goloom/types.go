package goloom

type ConferenceType string

const (
	ConferenceTypeConference ConferenceType = "CONFERENCE"
	MediaPlatformGoloom     string           = "GOLOOM"
)

type CreateConferenceResponse struct {
	ConnectionType      ConferenceType      `json:"connection_type"`
	URI                 string              `json:"uri"`
	RoomID              string              `json:"room_id"`
	SafeRoomID          string              `json:"safe_room_id"`
	PeerID              string              `json:"peer_id"`
	ClientConfiguration ClientConfiguration `json:"client_configuration"`
	ConferenceState     ConferenceState     `json:"conference_state"`
	MediaPlatform       string              `json:"media_platform"`
	IsLegalEntity       bool                `json:"is_legal_entity"`
	SessionID           string              `json:"session_id"`
	WaitingRoomAvailable bool               `json:"waiting_room_available"`
	ExpirationTime      int64               `json:"expiration_time"`
	ConferenceLimit     int                 `json:"conference_limit"`
	PeerSessionID       string              `json:"peer_session_id"`
	Credentials         string              `json:"credentials"`
	WsURI               string              `json:"ws_uri"`
}

type ClientConfiguration struct {
	CloudRecordingAvailable         bool        `json:"cloud_recording_available"`
	WaitTimeToReconnectMs           int         `json:"wait_time_to_reconnect_ms"`
	MediaServerURL                  string      `json:"media_server_url"`
	ServiceName                     string      `json:"service_name"`
	AliceProEnabled                 bool        `json:"alice_pro_enabled"`
	SummarizationAvailable          bool        `json:"summarization_available"`
	NewGridMobile                   bool        `json:"new_grid_mobile"`
	ExtendedRolePermissionsEnabled  bool        `json:"extended_role_permissions_enabled"`
	CalendarSummarizationReceiveAvailable bool `json:"calendar_summarization_receive_available"`
	ReactionsAvailable              bool        `json:"reactions_available"`
	JoinURLHidden                   bool        `json:"join_url_hidden"`
	StateCheckIntervalSeconds       int         `json:"state_check_interval_seconds"`
	GoloomSessionOpenMs             int         `json:"goloom_session_open_ms"`
	NewGrid                         bool        `json:"new_grid"`
	WaitingRoomPeersRefreshIntervalMs int       `json:"waiting_room_peers_refresh_interval_ms"`
	ReportProblemButtonAvailable    bool        `json:"report_problem_button_available"`
	ICEServers                      []ICEServer `json:"ice_servers"`
}

type ICEServer struct {
	URLs []string `json:"urls"`
}

type ConferenceState struct {
	LocalRecordingAllowed                bool   `json:"local_recording_allowed"`
	CloudRecordingAllowed                bool   `json:"cloud_recording_allowed"`
	ChatAllowed                          bool   `json:"chat_allowed"`
	ControlAllowed                       bool   `json:"control_allowed"`
	BroadcastAllowed                     bool   `json:"broadcast_allowed"`
	BroadcastFeatureEnabled              bool   `json:"broadcast_feature_enabled"`
	AccessRestrictionOrganizationAllowed bool   `json:"access_restriction_organization_allowed"`
	AccessLevel                          string `json:"access_level"`
	SummarizationStatus                  string `json:"summarization_status"`
	CloudRecordingStatus                 string `json:"cloud_recording_status"`
}

type UserInfo struct {
	UID                string            `json:"uid"`
	DisplayName        string            `json:"display_name"`
	AvatarURL          string            `json:"avatar_url"`
	IsDefaultAvatar    bool              `json:"is_default_avatar"`
	AvatarPlaceholder  AvatarPlaceholder `json:"avatar_placeholder"`
	IsYandexStaff      bool              `json:"is_yandex_staff"`
	BroadcastAllowed   bool              `json:"broadcast_allowed"`
	BroadcastFeatureEnabled bool         `json:"broadcast_feature_enabled"`
	CanCreateOrganizationRestrictedConference bool `json:"can_create_organization_restricted_conference"`
	IsLegalEntity      bool              `json:"is_legal_entity"`
	NoiseCancellationFeatureEnabled bool `json:"noise_cancellation_feature_enabled"`
	ConferencesHistoryFeatureEnabled bool `json:"conferences_history_feature_enabled"`
	BandwidthEffective4kCodecExp bool   `json:"bandwidth_effective4k_codec_exp"`
	Login              string            `json:"login"`
	Country            string            `json:"country"`
	IsPdd              bool              `json:"is_pdd"`
}

type AvatarPlaceholder struct {
	BackgroundColor string `json:"background_color"`
	TextColor       string `json:"text_color"`
	Abbreviation    string `json:"abbreviation"`
}

type RequestStatesRequest struct {
	Peers       []PeerRef          `json:"peers"`
	Permissions map[string]any     `json:"permissions"`
	Conference  ConferenceVersion  `json:"conference"`
}

type PeerRef struct {
	PeerID string `json:"peer_id"`
}

type ConferenceVersion struct {
	Version int `json:"version"`
}

type RequestStatesResponse struct {
	Permissions PermissionsData  `json:"permissions"`
	Peers       []PeerState      `json:"peers"`
	Conference  ConferenceData   `json:"conference"`
}

type PermissionsData struct {
	Version                 int            `json:"version"`
	PublicRolePermissions   []RolePermission `json:"public_role_permissions"`
	PersonalAllowed         []string       `json:"personal_allowed"`
}

type RolePermission struct {
	Role     string   `json:"role"`
	Allowed  []string `json:"allowed"`
}

type PeerState struct {
	PeerID   string    `json:"peer_id"`
	PeerType string    `json:"peer_type"`
	Version  int       `json:"version"`
	State    PeerData  `json:"state"`
}

type PeerData struct {
	UserData UserData `json:"user_data"`
}

type UserData struct {
	Role               string            `json:"role"`
	UID                string            `json:"uid"`
	MssngrGUID         string            `json:"mssngr_guid"`
	DisplayName        string            `json:"display_name"`
	AvatarURL          string            `json:"avatar_url"`
	IsDefaultAvatar    bool              `json:"is_default_avatar"`
	AvatarPlaceholder  AvatarPlaceholder `json:"avatar_placeholder"`
}

type ConferenceData struct {
	Version int `json:"version"`
}
