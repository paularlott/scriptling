package telegram

// LibraryName is the import path used in Scriptling scripts.
const LibraryName = "scriptling.messaging.telegram"

// internal types used by client.go / parseUpdate

type fileInfo struct {
	FileID   string
	FileName string
	MimeType string
	FileSize int64
}

type rawUpdate struct {
	UpdateID     int64
	ChatID       int64
	UserID       string
	UserName     string
	Text         string
	IsCallback   bool
	CallbackID   string
	CallbackData string
	File         *fileInfo
}
