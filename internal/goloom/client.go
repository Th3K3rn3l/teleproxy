package goloom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const (
	BaseURL     = "https://cloud-api.yandex.ru"
	APIEndpoint = "/telemost_front/v2/telemost"
	ClientVersion = "196.1.0"
)

type Client struct {
	httpClient     *http.Client
	uid            string
	clientInstance string
}

func NewClient(uid string) *Client {
	return &Client{
		httpClient:     &http.Client{},
		uid:            uid,
		clientInstance: newUUID(),
	}
}

func (c *Client) CreateConference() (*CreateConferenceResponse, error) {
	u := fmt.Sprintf("%s%s/conferences?next_gen_media_platform_allowed=true", BaseURL, APIEndpoint)
	body := bytes.NewReader([]byte("{}"))

	req, err := http.NewRequest("POST", u, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("Idempotency-Key", newUUID())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(b))
	}

	var conf CreateConferenceResponse
	if err := json.NewDecoder(resp.Body).Decode(&conf); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &conf, nil
}

func (c *Client) JoinConference(conferenceURI string) (*CreateConferenceResponse, error) {
	encoded := url.QueryEscape(conferenceURI)
	u := fmt.Sprintf("%s%s/conferences/%s/connection?next_gen_media_platform_allowed=true&waiting_room_supported=true",
		BaseURL, APIEndpoint, encoded)

	req, err := http.NewRequest("POST", u, bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("Idempotency-Key", newUUID())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(b))
	}

	var conf CreateConferenceResponse
	if err := json.NewDecoder(resp.Body).Decode(&conf); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &conf, nil
}

func (c *Client) RequestStates(conferenceURI string, peerIDs []string, stateVersion int) (*RequestStatesResponse, error) {
	encoded := url.QueryEscape(conferenceURI)
	u := fmt.Sprintf("%s%s/conferences/%s/request-states", BaseURL, APIEndpoint, encoded)

	peers := make([]PeerRef, len(peerIDs))
	for i, pid := range peerIDs {
		peers[i] = PeerRef{PeerID: pid}
	}

	reqBody := RequestStatesRequest{
		Peers:       peers,
		Permissions: map[string]any{},
		Conference:  ConferenceVersion{Version: stateVersion},
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", u, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("Idempotency-Key", newUUID())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(b))
	}

	var state RequestStatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &state, nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Client-Instance-Id", c.clientInstance)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://telemost.yandex.ru")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", "https://telemost.yandex.ru/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("X-Telemost-Client-Version", ClientVersion)
	req.Header.Set("X-Uid", c.uid)
}
