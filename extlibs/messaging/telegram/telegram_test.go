package telegram

import (
	"encoding/base64"
	"testing"
)

func TestParseUpdate_TextMessage(t *testing.T) {
	raw := map[string]interface{}{
		"update_id": float64(100),
		"message": map[string]interface{}{
			"chat": map[string]interface{}{"id": float64(42)},
			"from": map[string]interface{}{
				"id":         float64(99),
				"first_name": "Alice",
				"last_name":  "Smith",
			},
			"text": "hello world",
		},
	}
	u := parseUpdate(raw)
	if u.UpdateID != 100 {
		t.Errorf("UpdateID: got %d, want 100", u.UpdateID)
	}
	if u.ChatID != 42 {
		t.Errorf("ChatID: got %d, want 42", u.ChatID)
	}
	if u.UserID != "99" {
		t.Errorf("UserID: got %q, want \"99\"", u.UserID)
	}
	if u.UserName != "Alice Smith" {
		t.Errorf("UserName: got %q, want \"Alice Smith\"", u.UserName)
	}
	if u.Text != "hello world" {
		t.Errorf("Text: got %q, want \"hello world\"", u.Text)
	}
	if u.IsCallback {
		t.Error("IsCallback should be false")
	}
	if u.File != nil {
		t.Error("File should be nil")
	}
}

func TestParseUpdate_CallbackQuery(t *testing.T) {
	raw := map[string]interface{}{
		"update_id": float64(200),
		"callback_query": map[string]interface{}{
			"id":   "cb123",
			"data": "menu_opt1",
			"from": map[string]interface{}{"id": float64(77), "first_name": "Bob"},
			"message": map[string]interface{}{
				"chat": map[string]interface{}{"id": float64(55)},
			},
		},
	}
	u := parseUpdate(raw)
	if !u.IsCallback {
		t.Error("IsCallback should be true")
	}
	if u.CallbackID != "cb123" {
		t.Errorf("CallbackID: got %q", u.CallbackID)
	}
	if u.CallbackData != "menu_opt1" {
		t.Errorf("CallbackData: got %q", u.CallbackData)
	}
	if u.UserID != "77" {
		t.Errorf("UserID: got %q", u.UserID)
	}
	if u.ChatID != 55 {
		t.Errorf("ChatID: got %d, want 55", u.ChatID)
	}
}

func TestParseUpdate_Document(t *testing.T) {
	raw := map[string]interface{}{
		"update_id": float64(300),
		"message": map[string]interface{}{
			"chat": map[string]interface{}{"id": float64(10)},
			"from": map[string]interface{}{"id": float64(20), "first_name": "Carol"},
			"document": map[string]interface{}{
				"file_id":   "doc_file_id",
				"file_name": "report.pdf",
				"mime_type": "application/pdf",
				"file_size": float64(1024),
			},
		},
	}
	u := parseUpdate(raw)
	if u.File == nil {
		t.Fatal("File should not be nil")
	}
	if u.File.FileID != "doc_file_id" {
		t.Errorf("File.FileID: got %q", u.File.FileID)
	}
	if u.File.MimeType != "application/pdf" {
		t.Errorf("File.MimeType: got %q", u.File.MimeType)
	}
	if u.File.FileSize != 1024 {
		t.Errorf("File.FileSize: got %d", u.File.FileSize)
	}
}

func TestParseUpdate_EditedMessage(t *testing.T) {
	raw := map[string]interface{}{
		"update_id": float64(500),
		"edited_message": map[string]interface{}{
			"chat": map[string]interface{}{"id": float64(11)},
			"from": map[string]interface{}{"id": float64(22), "first_name": "Eve"},
			"text": "edited text",
		},
	}
	u := parseUpdate(raw)
	if u.ChatID != 11 {
		t.Errorf("ChatID: got %d", u.ChatID)
	}
	if u.Text != "edited text" {
		t.Errorf("Text: got %q", u.Text)
	}
}

func TestBuildName_FirstAndLast(t *testing.T) {
	from := map[string]interface{}{"id": float64(1), "first_name": "John", "last_name": "Doe"}
	if got := buildName(from); got != "John Doe" {
		t.Errorf("got %q, want \"John Doe\"", got)
	}
}

func TestBuildName_FallbackToID(t *testing.T) {
	from := map[string]interface{}{"id": float64(12345)}
	if got := buildName(from); got != "12345" {
		t.Errorf("got %q, want \"12345\"", got)
	}
}

func TestResolveMedia_Base64(t *testing.T) {
	original := []byte("hello binary data")
	encoded := base64.StdEncoding.EncodeToString(original)
	data, isURL, fileID, err := resolveMedia(encoded, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isURL || fileID != "" {
		t.Error("expected raw bytes result")
	}
	if string(data) != string(original) {
		t.Errorf("decoded mismatch")
	}
}

func TestResolveMedia_URL(t *testing.T) {
	_, isURL, _, err := resolveMedia("https://example.com/photo.jpg", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isURL {
		t.Error("isURL should be true")
	}
}

func TestResolveMedia_FileID(t *testing.T) {
	data, isURL, fileID, err := resolveMedia("AgACAgIAAxkBAAIBf2Z", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isURL || data != nil {
		t.Error("expected fileID result")
	}
	if fileID != "AgACAgIAAxkBAAIBf2Z" {
		t.Errorf("fileID: got %q", fileID)
	}
}

func TestNormalise_CommandParsed(t *testing.T) {
	u := &rawUpdate{ChatID: 42, UserID: "1", Text: "/echo hello world"}
	n := normalise(u)
	if n.Command != "/echo" {
		t.Errorf("Command: got %q, want \"/echo\"", n.Command)
	}
	if len(n.Args) != 2 || n.Args[0] != "hello" {
		t.Errorf("Args: got %v", n.Args)
	}
	if n.Dest != "42" {
		t.Errorf("Dest: got %q, want \"42\"", n.Dest)
	}
}

func TestNormalise_PlainText(t *testing.T) {
	u := &rawUpdate{ChatID: 7, UserID: "2", Text: "just a message"}
	n := normalise(u)
	if n.Command != "" {
		t.Errorf("Command should be empty for plain text, got %q", n.Command)
	}
}
