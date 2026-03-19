package discord

import "encoding/json"

// LibraryName is the import path used in Scriptling scripts.
const LibraryName = "scriptling.messaging.discord"

// Gateway opcodes
const (
	opDispatch       = 0
	opHeartbeat      = 1
	opIdentify       = 2
	opResume         = 6
	opReconnect      = 7
	opInvalidSession = 9
	opHello          = 10
	opHeartbeatACK   = 11
)

const intentDirectMessages = 1 << 12

const (
	apiBase    = "https://discord.com/api/v10"
	gatewayURL = "wss://gateway.discord.gg/?v=10&encoding=json"
)

// rawUpdate is a parsed Discord event before normalisation.
type rawUpdate struct {
	ChannelID     string
	UserID        string
	UserName      string
	Text          string
	MessageID     string
	IsCallback    bool
	CallbackID    string
	CallbackToken string
	CallbackData  string
	File          *fileInfo
	Attachments   []fileInfo
}

type fileInfo struct {
	ID       string
	Name     string
	MimeType string
	Size     int64
	URL      string
}

// gatewayPayload is the raw Discord gateway message.
type gatewayPayload struct {
	Op   int             `json:"op"`
	Data json.RawMessage `json:"d"`
	Seq  *int64          `json:"s"`
	Type string          `json:"t"`
}
