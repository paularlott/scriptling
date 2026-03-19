package slack

// LibraryName is the import path used in Scriptling scripts.
const LibraryName = "scriptling.messaging.slack"

const apiBase = "https://slack.com/api/"

// rawUpdate is a parsed Slack event before normalisation.
type rawUpdate struct {
	ChannelID     string
	UserID        string
	UserName      string
	Text          string
	MessageTS     string // Slack message timestamp — used as message ID
	IsCallback    bool
	CallbackID    string   // interaction payload action_id
	CallbackToken string   // response_url or trigger_id
	CallbackData  string   // action value
	File          *fileInfo
}

type fileInfo struct {
	ID       string
	Name     string
	MimeType string
	Size     int64
	URL      string
}
