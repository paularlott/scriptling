package mcp_test

import (
	"context"
	"sync/atomic"
	"testing"

	mcplib "github.com/paularlott/mcp"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/mcp"
	"github.com/paularlott/scriptling/stdlib"
)

// elicitScriptServer serves script as the tool "s"; runs counts its runs.
func elicitScriptServer(t *testing.T, script string, runs *atomic.Int32) *mcplib.Server {
	t.Helper()
	srv := mcplib.NewServer("t", "1")
	srv.RegisterTool(mcplib.NewTool("s", "script", mcplib.String("name", "name")), func(ctx context.Context, req *mcplib.ToolRequest) (*mcplib.ToolResponse, error) {
		runs.Add(1)
		sl := scriptling.New()
		stdlib.RegisterAll(sl)
		mcp.RegisterToolHelpers(sl)
		name, _ := req.String("name")
		resp, _, err := mcp.RunToolScript(ctx, sl, script, map[string]any{"name": name})
		if err != nil {
			return nil, err
		}
		return mcplib.NewToolResponseText(resp), nil
	})
	return srv
}

func text(t *testing.T, resp *mcplib.ToolResponse) string {
	t.Helper()
	if resp == nil || len(resp.Content) == 0 {
		t.Fatalf("no response: %v", resp)
	}
	return resp.Content[0].Text
}

const confirmScript = `
import scriptling.mcp.tool as tool
name = tool.get_string("name")
plan = tool.recall("plan")
fresh = plan == None
if fresh:
    plan = {"summary": "create " + name}
    tool.remember("plan", plan)
ok = tool.confirm("Apply " + plan["summary"] + "?")
tool.return_object({"can_ask": tool.can_ask(), "ok": ok, "fresh": fresh})
`

func TestConfirmThroughElicitation(t *testing.T) {
	var runs atomic.Int32
	srv := elicitScriptServer(t, confirmScript, &runs)
	ctx := context.Background()
	args := map[string]any{"name": "Acme"}

	_, need, err := srv.CallToolWithInput(ctx, "s", args, nil)
	if err != nil || need == nil || len(need.Requests) != 1 {
		t.Fatalf("first run: %+v %v", need, err)
	}
	var key string
	for k, q := range need.Requests {
		key = k
		if q.Message != "Apply create Acme?" {
			t.Fatalf("asked %q", q.Message)
		}
	}
	for answer, want := range map[*mcplib.ElicitResult]string{
		{Action: mcplib.ElicitAccept, Content: map[string]any{"approve": true}}:  `{"can_ask":true,"fresh":false,"ok":true}`,
		{Action: mcplib.ElicitAccept, Content: map[string]any{"approve": false}}: `{"can_ask":true,"fresh":false,"ok":false}`,
		{Action: mcplib.ElicitDecline}:                                           `{"can_ask":true,"fresh":false,"ok":false}`,
	} {
		resp, need, err := srv.CallToolWithInput(ctx, "s", args, &mcplib.ToolInput{Responses: map[string]*mcplib.ElicitResult{key: answer}, State: need.State})
		if err != nil || need != nil {
			t.Fatalf("answer %+v: %+v %v", answer, need, err)
		}
		if got := text(t, resp); got != want {
			t.Fatalf("answer %+v: %s, want %s", answer, got, want)
		}
	}

	// A caller that cannot be asked: confirm() is False at once, nothing kept.
	resp, err := srv.CallTool(ctx, "s", args)
	if err != nil || text(t, resp) != `{"can_ask":false,"fresh":true,"ok":false}` {
		t.Fatalf("CallTool: %v %v", resp, err)
	}
}

const askScript = `
import scriptling.mcp.tool as tool
answer = tool.ask("Which environment?", {
    "type": "object",
    "properties": {"env": {"type": "string", "enum": ["staging", "prod"]}},
    "required": ["env"],
}, key="env")
tool.return_object({"answer": answer})
`

func TestAskThroughElicitation(t *testing.T) {
	var runs atomic.Int32
	srv := elicitScriptServer(t, askScript, &runs)
	ctx := context.Background()

	_, need, err := srv.CallToolWithInput(ctx, "s", nil, nil)
	if err != nil || need == nil || need.Requests["env"].Message != "Which environment?" {
		t.Fatalf("first run: %+v %v", need, err)
	}
	resp, _, err := srv.CallToolWithInput(ctx, "s", nil, &mcplib.ToolInput{
		Responses: map[string]*mcplib.ElicitResult{"env": {Action: mcplib.ElicitAccept, Content: map[string]any{"env": "prod"}}},
		State:     need.State,
	})
	if err != nil || text(t, resp) != `{"answer":{"action":"accept","content":{"env":"prod"}}}` {
		t.Fatalf("answered: %v %v", resp, err)
	}
	if resp, err := srv.CallTool(ctx, "s", nil); err != nil || text(t, resp) != `{"answer":null}` {
		t.Fatalf("cannot ask: %v %v", resp, err)
	}
	if runs.Load() != 3 {
		t.Fatalf("runs = %d", runs.Load())
	}
}

// An answer belongs to its question: a re-run that asks about something
// else is asked again rather than given the earlier answer.
func TestAskAnswerIsNotReusedForAnotherQuestion(t *testing.T) {
	var runs atomic.Int32
	var target atomic.Value
	target.Store("invoice-1")
	srv := mcplib.NewServer("t", "1")
	srv.RegisterTool(mcplib.NewTool("s", "script"), func(ctx context.Context, req *mcplib.ToolRequest) (*mcplib.ToolResponse, error) {
		runs.Add(1)
		sl := scriptling.New()
		stdlib.RegisterAll(sl)
		mcp.RegisterToolHelpers(sl)
		script := `
import scriptling.mcp.tool as tool
ok = tool.confirm("Delete ` + target.Load().(string) + `?")
tool.return_object({"ok": ok})
`
		resp, _, err := mcp.RunToolScript(ctx, sl, script, map[string]any{})
		if err != nil {
			return nil, err
		}
		return mcplib.NewToolResponseText(resp), nil
	})
	ctx := context.Background()
	_, need, err := srv.CallToolWithInput(ctx, "s", nil, nil)
	if err != nil || need == nil {
		t.Fatal(need, err)
	}
	var key string
	for k := range need.Requests {
		key = k
	}
	target.Store("invoice-2") // the data changed before the re-run
	_, again, err := srv.CallToolWithInput(ctx, "s", nil, &mcplib.ToolInput{
		Responses: map[string]*mcplib.ElicitResult{key: {Action: mcplib.ElicitAccept, Content: map[string]any{"approve": true}}},
		State:     need.State,
	})
	if err != nil || again == nil {
		t.Fatalf("the yes to invoice-1 was taken for invoice-2: need=%v err=%v", again, err)
	}
	for _, q := range again.Requests {
		if q.Message != "Delete invoice-2?" {
			t.Fatalf("asked %q", q.Message)
		}
	}
}

// Integer answers reach the script as ints; the same confirm() may be asked
// again in one call; a question nobody answers in time counts as no.
func TestAskTypesRepeatsAndTimeouts(t *testing.T) {
	var runs atomic.Int32
	srv := elicitScriptServer(t, `
import scriptling.mcp.tool as tool
a = tool.ask("How many?", {"type": "object", "properties": {"n": {"type": "integer"}, "r": {"type": "number"}}, "required": ["n"]})
oks = [tool.confirm("Delete this one?"), tool.confirm("Delete this one?")]
tool.return_object({"n_type": type(a["content"]["n"]).__name__, "range": len(range(a["content"]["n"])), "r_type": type(a["content"]["r"]).__name__, "oks": oks})
`, &runs)
	answers := []*mcplib.ElicitResult{
		{Action: mcplib.ElicitAccept, Content: map[string]any{"n": float64(3), "r": 1.5}},
		{Action: mcplib.ElicitAccept, Content: map[string]any{"approve": true}},
		{Action: mcplib.ElicitAccept, Content: map[string]any{"approve": false}},
	}
	i := 0
	ctx := mcplib.WithElicitationHandler(context.Background(), func(ctx context.Context, req mcplib.ElicitationRequest) (*mcplib.ElicitResult, error) {
		res := answers[i]
		i++
		return res, nil
	})
	resp, err := srv.CallTool(ctx, "s", nil)
	if err != nil || text(t, resp) != `{"n_type":"int","oks":[true,false],"r_type":"float","range":3}` {
		t.Fatalf("got %v %v", resp, err)
	}

	timedOut := mcplib.WithElicitationHandler(context.Background(), func(context.Context, mcplib.ElicitationRequest) (*mcplib.ElicitResult, error) {
		return nil, mcplib.ErrElicitationTimeout
	})
	srv2 := elicitScriptServer(t, `
import scriptling.mcp.tool as tool
tool.return_object({"ok": tool.confirm("Go?"), "answer": tool.ask("Which?", {"type": "object", "properties": {"x": {"type": "string"}}})})
`, &runs)
	resp, err = srv2.CallTool(timedOut, "s", nil)
	if err != nil || text(t, resp) != `{"answer":{"action":"cancel"},"ok":false}` {
		t.Fatalf("timeout: %v %v", resp, err)
	}
}
