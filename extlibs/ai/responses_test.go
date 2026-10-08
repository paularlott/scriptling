package ai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	scriptlib "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

// weatherChatServer is a chat completions server for the Responses API
// emulation: while the conversation has no tool result it asks for the
// get_weather tool (when offered), otherwise it answers with the tool result
// and the system prompt it saw. It records each request's messages.
type weatherChatServer struct {
	mu       sync.Mutex
	requests [][]map[string]any
}

func (s *weatherChatServer) handler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Messages []map[string]any `json:"messages"`
		Tools    []any            `json:"tools"`
		Stream   bool             `json:"stream"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	s.requests = append(s.requests, req.Messages)
	s.mu.Unlock()

	var system, toolResult string
	for _, m := range req.Messages {
		switch m["role"] {
		case "system":
			system, _ = m["content"].(string)
		case "tool":
			toolResult, _ = m["content"].(string)
		}
	}

	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		chunk := map[string]any{"id": "c", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{
			"tool_calls": []any{map[string]any{"index": 0, "id": "call_s", "type": "function",
				"function": map[string]any{"name": "get_weather", "arguments": `{"city":"Oslo"}`}}},
		}}}}
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		fmt.Fprint(w, `data: {"id":"c","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	message := map[string]any{"role": "assistant", "content": fmt.Sprintf("result=%s system=%s", toolResult, system)}
	if toolResult == "" && len(req.Tools) > 0 {
		message = map[string]any{"role": "assistant", "content": "", "tool_calls": []any{map[string]any{
			"id": "call_1", "type": "function",
			"function": map[string]any{"name": "get_weather", "arguments": `{"city":"Paris"}`},
		}}}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": "chat_1", "object": "chat.completion",
		"choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": "stop"}},
	})
}

func newResponsesScript(t *testing.T, serverURL string) *scriptlib.Scriptling {
	t.Helper()
	p := scriptlib.New()
	Register(p, &netsecurity.Config{AllowIPLiterals: true, AllowLoopback: true})
	if err := p.SetVar("server_url", serverURL); err != nil {
		t.Fatal(err)
	}
	return p
}

// A script runs the Responses API tool loop: response_create with tools, run
// the calls with the registry, send ai.tool_outputs() back with
// previous_response_id, and read the answer with ai.text().
func TestResponsesToolLoopFromScript(t *testing.T) {
	srv := &weatherChatServer{}
	server := httptest.NewServer(http.HandlerFunc(srv.handler))
	defer server.Close()
	p := newResponsesScript(t, server.URL)

	result, err := p.Eval(`
import scriptling.ai as ai

def get_weather(args):
    return "sunny in " + args["city"]

tools = ai.ToolRegistry()
tools.add("get_weather", "Weather for a city", {"city": "string"}, get_weather)

client = ai.Client(server_url + "/v1")
emulated = client.supports("responses_emulated") and not client.supports("responses")

first = client.response_create("m", "Weather in Paris?", tools=tools.build(), instructions="Be brief")
calls = ai.tool_calls(first)
outputs = ai.tool_outputs(ai.execute_tool_calls(tools, calls))
second = client.response_create("m", outputs, previous_response_id=first.id, tools=tools.build())

[emulated, calls[0]["id"], calls[0]["function"]["name"], outputs[0]["type"], outputs[0]["call_id"], ai.text(second)]
`)
	if err != nil {
		t.Fatal(err)
	}
	got := result.Inspect()
	for _, want := range []string{"True", "call_1", "get_weather", "function_call_output", "result=sunny in Paris"} {
		if !strings.Contains(got, want) {
			t.Errorf("result %s missing %q", got, want)
		}
	}
	// Instructions apply to the first request only, as on the native API
	if strings.Contains(got, "system=Be brief") {
		t.Errorf("instructions carried over to the continued request: %s", got)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	if len(srv.requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(srv.requests))
	}
	if first := srv.requests[0]; first[0]["role"] != "system" || first[0]["content"] != "Be brief" {
		t.Errorf("first request messages = %v, want the instructions first", first)
	}
	// The continued request replays the stored conversation: the question,
	// the assistant's tool call, and the tool result
	roles := []string{}
	for _, m := range srv.requests[1] {
		roles = append(roles, fmt.Sprint(m["role"]))
	}
	if strings.Join(roles, ",") != "user,assistant,tool" {
		t.Errorf("continued request roles = %v, want user,assistant,tool", roles)
	}
}

func TestResponsesStreamToolCallsFromScript(t *testing.T) {
	srv := &weatherChatServer{}
	server := httptest.NewServer(http.HandlerFunc(srv.handler))
	defer server.Close()
	p := newResponsesScript(t, server.URL)

	result, err := p.Eval(`
import scriptling.ai as ai

tools = ai.ToolRegistry()
tools.add("get_weather", "Weather for a city", {"city": "string"}, lambda args: "x")

client = ai.Client(server_url + "/v1")
stream = client.response_stream("m", "Weather in Oslo?", tools=tools.build())
done = None
while True:
    event = stream.next()
    if event is None:
        break
    if event.type == "response.completed":
        done = event.response
calls = ai.tool_calls(done)
[len(calls), calls[0]["id"], calls[0]["function"]["arguments"]]
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); !strings.Contains(got, "1") || !strings.Contains(got, "call_s") || !strings.Contains(got, "Oslo") {
		t.Errorf("streamed tool calls = %s", got)
	}
}

func TestResponsesCompactStoreAndGrokFromScript(t *testing.T) {
	srv := &weatherChatServer{}
	server := httptest.NewServer(http.HandlerFunc(srv.handler))
	defer server.Close()
	p := newResponsesScript(t, server.URL)

	result, err := p.Eval(`
import scriptling.ai as ai

client = ai.Client(server_url + "/v1")
first = client.response_create("m", "My name is Zorblat")
compacted = client.response_compact("m", previous_response_id=first.id)

private = client.response_create("m", "secret", store=False)
retrievable = True
try:
    client.response_get(private.id)
except Exception:
    retrievable = False

grok = ai.Client("https://api.x.ai/v1", provider=ai.GROK, api_key="k")
[compacted.object, len(compacted.output), retrievable, ai.GROK, grok.supports("responses"), grok.supports("embeddings")]
`)
	if err != nil {
		t.Fatal(err)
	}
	want := `['response.compaction', 1, False, 'grok', True, False]`
	if got := result.Inspect(); got != want {
		t.Errorf("result = %s, want %s", got, want)
	}
}

func TestResponsesArgumentValidation(t *testing.T) {
	p := newResponsesScript(t, "http://127.0.0.1:1")
	for name, script := range map[string]string{
		"store not bool":     `client.response_create("m", "x", store="no")`,
		"compact nothing":    `client.response_compact("m")`,
		"tool_outputs no id": `ai.tool_outputs([{"content": "x"}])`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := p.Eval("import scriptling.ai as ai\nclient = ai.Client(\"http://127.0.0.1:1/v1\")\n" + script)
			if err == nil {
				t.Error("expected an error")
			}
		})
	}
}

// ai.text() and ai.tool_calls() read Responses API objects too.
func TestTextAndToolCallsReadResponsesObjects(t *testing.T) {
	resp := map[string]any{"object": "response", "output": []any{
		map[string]any{"type": "message", "content": []any{
			map[string]any{"type": "output_text", "text": "<think>hmm</think>Hello "},
			map[string]any{"type": "output_text", "text": "there"},
		}},
		map[string]any{"type": "function_call", "id": "fc_1", "call_id": "call_9", "name": "f", "arguments": `{"a":1}`},
	}}
	if got := responseText(resp); got != "Hello there" {
		t.Errorf("text = %q", got)
	}
	calls := extractToolCallsFromGo(resp)
	if len(calls) != 1 || calls[0]["id"] != "call_9" || calls[0]["function"].(map[string]any)["name"] != "f" {
		t.Errorf("tool calls = %v", calls)
	}
}

func TestToolsWithoutNameRejected(t *testing.T) {
	p := newResponsesScript(t, "http://127.0.0.1:1")
	for name, call := range map[string]string{
		"responses flat":    `client.response_create("m", "x", tools=[{"type": "function"}])`,
		"completion":        `client.completion("m", "x", tools=[{"type": "function", "function": {"description": "d"}}])`,
		"completion_stream": `client.completion_stream("m", "x", tools=[{"type": "function", "function": {}}])`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := p.Eval("import scriptling.ai as ai\nclient = ai.Client(\"http://127.0.0.1:1/v1\")\n" + call)
			if err == nil || !strings.Contains(err.Error(), "no function name") {
				t.Errorf("error = %v, want the missing tool name", err)
			}
		})
	}
}
