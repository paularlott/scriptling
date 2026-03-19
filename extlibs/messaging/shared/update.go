package shared

// RichMessage is a platform-agnostic rich content message.
// Each client translates it to the native format.
type RichMessage struct {
	Title string // bold heading
	Body  string // main text
	Color string // "red", "green", "blue", hex "#ff0000" — Discord embed color; ignored elsewhere
	Image string // URL or file path — attached as photo/embed image
	URL   string // click-through link — Discord embed URL; ignored elsewhere
}

// KeyboardButton is a single button in a platform-agnostic keyboard.
// If URL is set it is a link button; otherwise Data is the callback payload.
type KeyboardButton struct {
	Text string
	Data string // callback_data (Telegram) / custom_id (Discord)
	URL  string // link button — opens URL instead of firing a callback
}

// Keyboard is a platform-agnostic button grid.
// Each inner slice is a row of buttons.
type Keyboard [][]KeyboardButton

// Update is the normalised event passed to all handlers regardless of platform.
// Dest is the reply target — chat_id (Telegram) or channel_id (Discord/Slack) as a string.
type Update struct {
	Dest          string
	MessageID     string
	UserID        string
	UserName      string
	Text          string
	Command       string   // "/start" if text begins with /, else ""
	Args          []string // words after the command
	IsCallback    bool
	CallbackID    string
	CallbackToken string // Discord/Slack interaction token; empty on Telegram
	CallbackData  string
	File          *FileInfo
}

// FileInfo holds normalised file metadata.
type FileInfo struct {
	ID       string
	Name     string
	MimeType string
	Size     int64
	URL      string // download URL (Discord/Slack); empty for Telegram (use ID)
}
