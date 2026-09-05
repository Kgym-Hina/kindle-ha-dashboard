package ha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type State struct {
	EntityID   string         `json:"entity_id"`
	State      string         `json:"state"`
	Attributes map[string]any `json:"attributes"`
}

type Message struct {
	TargetDeviceID string `json:"target_device_id,omitempty"`
	Title          string `json:"title"`
	Message        string `json:"message"`
	TimeoutMS      int    `json:"timeout_ms,omitempty"`
}

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	mu         sync.RWMutex
	conn       *websocket.Conn
	writeMu    sync.Mutex
	messageID  atomic.Int64
}

func NewClient(baseURL, token string) *Client {
	client := &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:      strings.TrimSpace(token),
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
	client.messageID.Store(10)
	return client
}

func (c *Client) FetchState(ctx context.Context, entityID string) (*State, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL("/api/states/"+url.PathEscape(entityID)), nil)
	if err != nil {
		return nil, err
	}
	c.authorize(request)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		return nil, fmt.Errorf("get state %s: %s: %s", entityID, response.Status, strings.TrimSpace(string(body)))
	}
	var state State
	if err := json.NewDecoder(response.Body).Decode(&state); err != nil {
		return nil, fmt.Errorf("decode state %s: %w", entityID, err)
	}
	return &state, nil
}

func (c *Client) UpdateState(ctx context.Context, entityID, state string, attributes map[string]any) error {
	body, err := json.Marshal(map[string]any{"state": state, "attributes": attributes})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL("/api/states/"+url.PathEscape(entityID)), strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	c.authorize(request)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		return fmt.Errorf("update state %s: %s: %s", entityID, response.Status, strings.TrimSpace(string(responseBody)))
	}
	return nil
}

func (c *Client) CallService(ctx context.Context, domain, service string, serviceData map[string]any) error {
	if strings.TrimSpace(domain) == "" || strings.TrimSpace(service) == "" {
		return errors.New("call_service requires domain and service")
	}
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()
	if conn == nil {
		return errors.New("home assistant websocket is not connected")
	}
	if serviceData == nil {
		serviceData = map[string]any{}
	}
	message := map[string]any{
		"id":           c.nextID(),
		"type":         "call_service",
		"domain":       domain,
		"service":      service,
		"service_data": serviceData,
	}
	return c.write(conn, message)
}

func (c *Client) Run(ctx context.Context, onState func(State), onMessage func(Message)) error {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		conn, err := c.connect(ctx)
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
		c.mu.Lock()
		c.conn = conn
		c.mu.Unlock()

		err = c.consume(ctx, conn, onState, onMessage)
		c.mu.Lock()
		if c.conn == conn {
			c.conn = nil
		}
		c.mu.Unlock()
		_ = conn.Close()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
	}
}

func (c *Client) connect(ctx context.Context) (*websocket.Conn, error) {
	websocketURL, err := c.websocketURL()
	if err != nil {
		return nil, err
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, websocketURL, nil)
	if err != nil {
		return nil, err
	}
	if err := c.write(conn, map[string]any{"type": "auth", "access_token": c.token}); err != nil {
		_ = conn.Close()
		return nil, err
	}
	var auth struct {
		Type string `json:"type"`
	}
	if err := conn.ReadJSON(&auth); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if auth.Type != "auth_ok" {
		_ = conn.Close()
		return nil, fmt.Errorf("home assistant authentication failed: %s", auth.Type)
	}
	for _, eventType := range []string{"state_changed", "kindle_dashboard_message"} {
		if err := c.subscribe(conn, eventType); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}
	return conn, nil
}

func (c *Client) subscribe(conn *websocket.Conn, eventType string) error {
	if err := c.write(conn, map[string]any{"id": c.nextID(), "type": "subscribe_events", "event_type": eventType}); err != nil {
		return err
	}
	for {
		var response struct {
			Type    string `json:"type"`
			Success bool   `json:"success"`
			Error   any    `json:"error"`
		}
		if err := conn.ReadJSON(&response); err != nil {
			return err
		}
		if response.Type != "result" {
			continue
		}
		if !response.Success {
			return fmt.Errorf("subscribe %s failed: %v", eventType, response.Error)
		}
		return nil
	}
}

func (c *Client) consume(ctx context.Context, conn *websocket.Conn, onState func(State), onMessage func(Message)) error {
	stopPing := make(chan struct{})
	defer close(stopPing)
	go c.pingLoop(ctx, conn, stopPing)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var envelope struct {
			Type  string          `json:"type"`
			Event json.RawMessage `json:"event"`
		}
		if err := conn.ReadJSON(&envelope); err != nil {
			return err
		}
		if envelope.Type != "event" {
			continue
		}
		var event struct {
			EventType string `json:"event_type"`
			Data      struct {
				EntityID       string `json:"entity_id"`
				NewState       *State `json:"new_state"`
				TargetDeviceID string `json:"target_device_id"`
				Title          string `json:"title"`
				Message        string `json:"message"`
				TimeoutMS      int    `json:"timeout_ms"`
			} `json:"data"`
		}
		if err := json.Unmarshal(envelope.Event, &event); err != nil {
			continue
		}
		switch event.EventType {
		case "state_changed":
			if event.Data.NewState != nil && onState != nil {
				onState(*event.Data.NewState)
			}
		case "kindle_dashboard_message":
			if onMessage != nil {
				onMessage(Message{
					TargetDeviceID: event.Data.TargetDeviceID,
					Title:          event.Data.Title,
					Message:        event.Data.Message,
					TimeoutMS:      event.Data.TimeoutMS,
				})
			}
		}
	}
}

func (c *Client) pingLoop(ctx context.Context, conn *websocket.Conn, stop <-chan struct{}) {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-ticker.C:
			_ = c.writeControl(conn, websocket.PingMessage, []byte("kindle-dashboard"))
		}
	}
}

func (c *Client) write(conn *websocket.Conn, payload any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return conn.WriteJSON(payload)
}

func (c *Client) writeControl(conn *websocket.Conn, messageType int, data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return conn.WriteControl(messageType, data, time.Now().Add(5*time.Second))
}

func (c *Client) authorize(request *http.Request) {
	request.Header.Set("Authorization", "Bearer "+c.token)
}

func (c *Client) apiURL(path string) string {
	return c.baseURL + "/" + strings.TrimLeft(path, "/")
}

func (c *Client) websocketURL() (string, error) {
	parsed, err := url.Parse(c.baseURL + "/api/websocket")
	if err != nil {
		return "", err
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	}
	return parsed.String(), nil
}

func (c *Client) nextID() int64 {
	return c.messageID.Add(1)
}
