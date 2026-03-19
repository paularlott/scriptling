package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

// socketPayload is a raw Socket Mode envelope.
type socketPayload struct {
	EnvelopeID string          `json:"envelope_id"`
	Type       string          `json:"type"`
	Payload    json.RawMessage `json:"payload"`
	Accepts    bool            `json:"accepts_response_payload"`
}

type socketGateway struct {
	c       *slackClient
	updates chan *rawUpdate
}

func newSocketGateway(c *slackClient) *socketGateway {
	return &socketGateway{c: c}
}

func (g *socketGateway) connect(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		if err := g.dial(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func (g *socketGateway) dial(ctx context.Context) error {
	url, err := g.c.openSocketConnection(ctx)
	if err != nil {
		return fmt.Errorf("slack: openSocketConnection: %w", err)
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("slack: gateway dial: %w", err)
	}
	defer conn.Close()

	g.c.log.Info("slack gateway connected")

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		var p socketPayload
		if err := json.Unmarshal(msg, &p); err != nil {
			continue
		}

		// Acknowledge every envelope immediately
		if p.EnvelopeID != "" {
			ack, _ := json.Marshal(map[string]string{"envelope_id": p.EnvelopeID})
			conn.WriteMessage(websocket.TextMessage, ack)
		}

		switch p.Type {
		case "hello":
			// connection established — nothing to do
		case "disconnect":
			g.c.log.Info("slack gateway: server requested disconnect, reconnecting")
			return fmt.Errorf("slack: server requested disconnect")
		case "events_api":
			u := g.parseEventsAPI(p.Payload)
			if u != nil {
				g.send(u)
			}
		case "interactive":
			u := g.parseInteractive(p.Payload)
			if u != nil {
				g.send(u)
			}
		}
	}
}

func (g *socketGateway) send(u *rawUpdate) {
	select {
	case g.updates <- u:
	default:
	}
}

func (g *socketGateway) parseEventsAPI(raw json.RawMessage) *rawUpdate {
	var env struct {
		Event struct {
			Type        string `json:"type"`
			ChannelType string `json:"channel_type"`
			Channel     string `json:"channel"`
			User        string `json:"user"`
			Text        string `json:"text"`
			TS          string `json:"ts"`
			BotID       string `json:"bot_id"`
			Files       []struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Mimetype string `json:"mimetype"`
				Size     int64  `json:"size"`
				URLPriv  string `json:"url_private"`
			} `json:"files"`
		} `json:"event"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil
	}
	e := env.Event
	if e.Type != "message" {
		return nil
	}
	// DM-only: ignore channel messages, group messages, etc.
	if e.ChannelType != "im" {
		return nil
	}
	// Ignore bot messages
	if e.BotID != "" || e.User == "" {
		return nil
	}
	u := &rawUpdate{
		ChannelID: e.Channel,
		UserID:    e.User,
		Text:      e.Text,
		MessageTS: e.TS,
	}
	if len(e.Files) > 0 {
		f := e.Files[0]
		u.File = &fileInfo{
			ID:       f.ID,
			Name:     f.Name,
			MimeType: f.Mimetype,
			Size:     f.Size,
			URL:      f.URLPriv,
		}
	}
	return u
}

func (g *socketGateway) parseInteractive(raw json.RawMessage) *rawUpdate {
	var p struct {
		Type        string `json:"type"`
		TriggerID   string `json:"trigger_id"`
		ResponseURL string `json:"response_url"`
		User        struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"user"`
		Channel struct {
			ID string `json:"id"`
		} `json:"channel"`
		Actions []struct {
			ActionID string `json:"action_id"`
			Value    string `json:"value"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil
	}
	if p.Type != "block_actions" || len(p.Actions) == 0 {
		return nil
	}
	a := p.Actions[0]
	return &rawUpdate{
		ChannelID:     p.Channel.ID,
		UserID:        p.User.ID,
		UserName:      p.User.Name,
		IsCallback:    true,
		CallbackID:    a.ActionID,
		CallbackToken: p.ResponseURL,
		CallbackData:  a.Value,
	}
}
