package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

type gateway struct {
	c           *discordClient
	conn        *websocket.Conn
	seq         int64
	sessionID   string
	resumeURL   string
	heartbeatMs int
	updates     chan *rawUpdate
	done        chan struct{}
}

func newGateway(c *discordClient) *gateway {
	return &gateway{
		c:    c,
		done: make(chan struct{}),
	}
}

func (g *gateway) connect(ctx context.Context) error {
	for {
		if err := g.dial(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
			}
			continue
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}

func (g *gateway) dial(ctx context.Context) error {
	url := g.resumeURL
	if url == "" {
		var err error
		url, err = g.c.getGatewayURL(ctx)
		if err != nil {
			return err
		}
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("gateway dial: %w", err)
	}
	g.conn = conn
	defer func() {
		conn.Close()
		g.conn = nil
	}()

	heartbeatDone := make(chan struct{})
	var heartbeatTicker *time.Ticker
	defer func() {
		close(heartbeatDone)
		if heartbeatTicker != nil {
			heartbeatTicker.Stop()
		}
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		var p gatewayPayload
		if err := json.Unmarshal(msg, &p); err != nil {
			continue
		}
		if p.Seq != nil {
			g.seq = *p.Seq
		}

		switch p.Op {
		case opHello:
			var hello struct {
				HeartbeatInterval int `json:"heartbeat_interval"`
			}
			json.Unmarshal(p.Data, &hello)
			g.heartbeatMs = hello.HeartbeatInterval
			heartbeatTicker = time.NewTicker(time.Duration(g.heartbeatMs) * time.Millisecond)
			go func() {
				for {
					select {
					case <-heartbeatDone:
						return
					case <-ctx.Done():
						return
					case <-heartbeatTicker.C:
						g.sendHeartbeat(conn)
					}
				}
			}()
			if g.sessionID != "" {
				g.sendResume(conn)
			} else {
				g.sendIdentify(conn)
			}

		case opDispatch:
			if p.Type == "READY" {
				var ready struct {
					SessionID string `json:"session_id"`
					ResumeURL string `json:"resume_gateway_url"`
					User      struct {
						ID string `json:"id"`
					} `json:"user"`
				}
				json.Unmarshal(p.Data, &ready)
				g.sessionID = ready.SessionID
				g.resumeURL = ready.ResumeURL + "?v=10&encoding=json"
				g.c.botUserID = ready.User.ID
				g.c.log.Info("gateway connected", "session", g.sessionID, "bot_id", g.c.botUserID)
				continue
			}
			if p.Type == "MESSAGE_CREATE" || p.Type == "INTERACTION_CREATE" {
				var data map[string]interface{}
				json.Unmarshal(p.Data, &data)
				if p.Type == "MESSAGE_CREATE" {
					if author, ok := data["author"].(map[string]interface{}); ok {
						if id, _ := author["id"].(string); id == g.c.botUserID {
							continue
						}
					}
				}
				u := parseEvent(p.Type, data)
				if u.ChannelID != "" || u.IsCallback {
					select {
					case g.updates <- u:
					default:
					}
				}
			}

		case opReconnect:
			g.c.log.Info("gateway: server requested reconnect")
			return fmt.Errorf("gateway: server requested reconnect")

		case opInvalidSession:
			var resumable bool
			json.Unmarshal(p.Data, &resumable)
			if !resumable {
				g.sessionID = ""
				g.resumeURL = ""
			}
			g.c.log.Info("gateway: invalid session, reconnecting", "resumable", resumable)
			return fmt.Errorf("gateway: invalid session (resumable=%v)", resumable)

		case opHeartbeat:
			g.sendHeartbeat(conn)
		}
	}
}

func (g *gateway) sendHeartbeat(conn *websocket.Conn) {
	var seq interface{}
	if g.seq > 0 {
		seq = g.seq
	}
	payload, _ := json.Marshal(map[string]interface{}{"op": opHeartbeat, "d": seq})
	conn.WriteMessage(websocket.TextMessage, payload)
}

func (g *gateway) sendIdentify(conn *websocket.Conn) {
	payload, _ := json.Marshal(map[string]interface{}{
		"op": opIdentify,
		"d": map[string]interface{}{
			"token":   g.c.token,
			"intents": intentDirectMessages,
			"properties": map[string]string{
				"os":      "linux",
				"browser": "scriptling",
				"device":  "scriptling",
			},
		},
	})
	conn.WriteMessage(websocket.TextMessage, payload)
}

func (g *gateway) sendResume(conn *websocket.Conn) {
	payload, _ := json.Marshal(map[string]interface{}{
		"op": opResume,
		"d": map[string]interface{}{
			"token":      g.c.token,
			"session_id": g.sessionID,
			"seq":        g.seq,
		},
	})
	conn.WriteMessage(websocket.TextMessage, payload)
}
