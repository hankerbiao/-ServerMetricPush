package controlplane

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type RegisterRequest struct {
	AgentID                string    `json:"agent_id"`
	Hostname               string    `json:"hostname"`
	Version                string    `json:"version"`
	OS                     string    `json:"os"`
	Arch                   string    `json:"arch"`
	IP                     string    `json:"ip,omitempty"`
	PushgatewayURL         string    `json:"pushgateway_url"`
	PushIntervalSeconds    int       `json:"push_interval_seconds"`
	NodeExporterPort       int       `json:"node_exporter_port"`
	NodeExporterMetricsURL string    `json:"node_exporter_metrics_url"`
	StartedAt              time.Time `json:"started_at"`
}

type RegisterResponse struct {
	HeartbeatIntervalSeconds int `json:"heartbeat_interval_seconds"`
	OfflineTimeoutSeconds    int `json:"offline_timeout_seconds"`
}

type HeartbeatRequest struct {
	AgentID           string     `json:"agent_id"`
	Status            string     `json:"status"`
	LastError         string     `json:"last_error,omitempty"`
	LastPushAt        *time.Time `json:"last_push_at,omitempty"`
	LastPushSuccessAt *time.Time `json:"last_push_success_at,omitempty"`
	LastPushErrorAt   *time.Time `json:"last_push_error_at,omitempty"`
	PushFailCount     int        `json:"push_fail_count"`
	NodeExporterUp    bool       `json:"node_exporter_up"`
}

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("control plane request failed: status=%d body=%s", e.StatusCode, e.Body)
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Register(payload RegisterRequest) error {
	var response RegisterResponse
	return c.postJSON("/api/agents/register", payload, &response)
}

func (c *Client) Heartbeat(payload HeartbeatRequest) error {
	return c.postJSON("/api/agents/heartbeat", payload, nil)
}

func (c *Client) postJSON(path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request failed: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payloadBody, _ := io.ReadAll(resp.Body)
		return &APIError{StatusCode: resp.StatusCode, Body: string(payloadBody)}
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response failed: %w", err)
	}

	return nil
}
