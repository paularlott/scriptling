package discord

import (
	"testing"

	"github.com/paularlott/scriptling/extlibs/messaging/shared"
	"github.com/paularlott/scriptling/object"
)

func TestParseEventMessageCreate(t *testing.T) {
	data := map[string]interface{}{
		"id":         "111",
		"channel_id": "222",
		"content":    "hello world",
		"author": map[string]interface{}{
			"id":          "333",
			"username":    "alice",
			"global_name": "Alice",
		},
		"attachments": []interface{}{},
	}
	u := parseEvent("MESSAGE_CREATE", data)
	if u.MessageID != "111" {
		t.Errorf("MessageID: got %q", u.MessageID)
	}
	if u.ChannelID != "222" {
		t.Errorf("ChannelID: got %q", u.ChannelID)
	}
	if u.Text != "hello world" {
		t.Errorf("Text: got %q", u.Text)
	}
	if u.UserID != "333" {
		t.Errorf("UserID: got %q", u.UserID)
	}
	if u.UserName != "Alice" {
		t.Errorf("UserName: got %q", u.UserName)
	}
	if u.IsCallback {
		t.Error("IsCallback should be false")
	}
	if u.File != nil {
		t.Error("File should be nil when no attachments")
	}
}

func TestParseEventMessageCreateWithAttachment(t *testing.T) {
	data := map[string]interface{}{
		"id":         "111",
		"channel_id": "222",
		"content":    "",
		"author":     map[string]interface{}{"id": "333", "username": "alice"},
		"attachments": []interface{}{
			map[string]interface{}{
				"id":           "att1",
				"filename":     "report.pdf",
				"url":          "https://cdn.discordapp.com/attachments/report.pdf",
				"content_type": "application/pdf",
				"size":         float64(12345),
			},
		},
	}
	u := parseEvent("MESSAGE_CREATE", data)
	if u.File == nil {
		t.Fatal("File should not be nil")
	}
	if u.File.Name != "report.pdf" {
		t.Errorf("File.Name: got %q", u.File.Name)
	}
	if u.File.Size != 12345 {
		t.Errorf("File.Size: got %d", u.File.Size)
	}
}

func TestParseEventInteractionCreate(t *testing.T) {
	data := map[string]interface{}{
		"id":         "inter1",
		"token":      "tok123",
		"channel_id": "chan1",
		"user":       map[string]interface{}{"id": "user1", "username": "bob"},
		"data":       map[string]interface{}{"custom_id": "btn_a"},
	}
	u := parseEvent("INTERACTION_CREATE", data)
	if !u.IsCallback {
		t.Error("IsCallback should be true")
	}
	if u.CallbackID != "inter1" {
		t.Errorf("CallbackID: got %q", u.CallbackID)
	}
	if u.CallbackToken != "tok123" {
		t.Errorf("CallbackToken: got %q", u.CallbackToken)
	}
	if u.CallbackData != "btn_a" {
		t.Errorf("CallbackData: got %q", u.CallbackData)
	}
}

func TestParseEventInteractionCreateMemberUser(t *testing.T) {
	data := map[string]interface{}{
		"id":         "inter2",
		"token":      "tok456",
		"channel_id": "chan2",
		"member": map[string]interface{}{
			"user": map[string]interface{}{"id": "user2", "username": "carol"},
		},
		"data": map[string]interface{}{"custom_id": "btn_b"},
	}
	u := parseEvent("INTERACTION_CREATE", data)
	if u.UserID != "user2" {
		t.Errorf("UserID from member.user: got %q", u.UserID)
	}
}

func TestBuildName(t *testing.T) {
	tests := []struct {
		user     map[string]interface{}
		expected string
	}{
		{map[string]interface{}{"global_name": "Alice Smith", "username": "alice"}, "Alice Smith"},
		{map[string]interface{}{"username": "bob"}, "bob"},
		{map[string]interface{}{"id": "999"}, "999"},
		{map[string]interface{}{}, ""},
	}
	for _, tt := range tests {
		got := buildName(tt.user)
		if got != tt.expected {
			t.Errorf("buildName(%v): got %q want %q", tt.user, got, tt.expected)
		}
	}
}

func TestResolveMediaBase64(t *testing.T) {
	data, isURL, err := resolveMedia("aGVsbG8=", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isURL {
		t.Error("isURL should be false")
	}
	if string(data) != "hello" {
		t.Errorf("decoded: got %q", string(data))
	}
}

func TestResolveMediaURL(t *testing.T) {
	_, isURL, err := resolveMedia("https://example.com/img.png", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isURL {
		t.Error("isURL should be true")
	}
}

func TestResolveMediaUnknown(t *testing.T) {
	_, _, err := resolveMedia("not-a-file-or-url", false)
	if err == nil {
		t.Error("expected error for unknown source")
	}
}

func TestNormalise_CommandParsed(t *testing.T) {
	u := &rawUpdate{ChannelID: "chan1", UserID: "u1", Text: "/echo hello world"}
	n := normalise(u)
	if n.Command != "/echo" {
		t.Errorf("Command: got %q", n.Command)
	}
	if len(n.Args) != 2 || n.Args[0] != "hello" {
		t.Errorf("Args: got %v", n.Args)
	}
	if n.Dest != "chan1" {
		t.Errorf("Dest: got %q", n.Dest)
	}
}

func TestObjectToNative(t *testing.T) {
	d := &object.Dict{Pairs: make(map[string]object.DictPair)}
	d.SetByString("label", &object.String{Value: "Click"})
	d.SetByString("style", object.NewInteger(1))
	native := shared.ObjectToNative(d)
	m, ok := native.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", native)
	}
	if m["label"] != "Click" {
		t.Errorf("label: got %v", m["label"])
	}
	if m["style"] != int64(1) {
		t.Errorf("style: got %v", m["style"])
	}
}
