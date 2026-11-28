package mcp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"math"
	"strings"

	mcplib "github.com/paularlott/mcp"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// Asking the person using the MCP client (elicitation) from a tool script:
// scriptling.mcp.tool.can_ask(), ask(), confirm(), remember() and recall().
//
// ask() goes through mcp.Elicit, so it works however the client is reached:
// a 2025-x client is asked mid-call and ask() waits for the answer; a
// 2026-07-28 client (and an application calling the tool in-process with
// CallToolWithInput, such as a chat UI) answers by calling the tool again,
// so the script stops at the question and runs again from the top with the
// answer. Read and check before asking, write only after the answer, and
// keep slow work for the re-run with remember().

// confirmSchema is confirm()'s form: one approve box, off by default.
var confirmSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"approve": map[string]any{
			"type":        "boolean",
			"title":       "Approve",
			"description": "Tick to allow this action.",
			"default":     false,
		},
	},
	"required": []string{"approve"},
}

// elicitBuiltins returns the elicitation functions of scriptling.mcp.tool.
func elicitBuiltins() map[string]*object.Builtin {
	return map[string]*object.Builtin{
		"can_ask": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				return object.NewBoolean(mcplib.ElicitationSupported(ctx))
			},
			HelpText: `can_ask() - Whether the person using the MCP client can be asked questions

True when the client supports elicitation for this call (ask() and confirm()
reach the person); False otherwise, when ask() returns None and confirm()
returns False.

Example:
  if not tool.can_ask():
      tool.return_error("Ask the user to confirm, then call again with confirm=true")`,
		},
		"ask": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				message, schemaObj, key, errObj := askArgs("ask", kwargs, args)
				if errObj != nil {
					return errObj
				}
				schema, ok := conversion.ToGo(schemaObj).(map[string]any)
				if !ok {
					return errors.NewError("ask(): schema must be a dict (a JSON Schema object)")
				}
				var res *mcplib.ElicitResult
				var exit object.Object
				if key != "" {
					res, exit = elicitOrStop(ctx, mcplib.ElicitForm{Key: key, Message: message, Schema: schema})
				} else {
					// Named after the question and its form, so an answer is
					// never taken for a different question on a re-run.
					canonical, _ := json.Marshal(schema)
					res, exit = elicitNamed(ctx, "ask-"+shortHash(message+"\x00"+string(canonical)), message, schema)
				}
				if exit != nil {
					return exit
				}
				if res == nil {
					return &object.Null{}
				}
				out := map[string]any{"action": string(res.Action)}
				if res.Action == mcplib.ElicitAccept {
					out["content"] = typedAnswer(schema, res.Content)
				}
				return conversion.FromGo(out)
			},
			HelpText: `ask(message, schema, key="") - Ask the person a question through the MCP client

Shows message with a form built from schema: a JSON Schema object of flat
properties (string, number, integer, boolean, string enums, multi-select
arrays of enums), as MCP elicitation allows.

Returns:
  dict - {"action": "accept", "content": {...}}, {"action": "decline"} or
         {"action": "cancel"}
  None - the client cannot be asked (see can_ask())

With some clients the tool runs again from the top to deliver the answer:
the script stops at the question and the next run gets the answer at once.
So read and check before asking, write only after the answer, and keep slow
work for that run with remember().

An answer belongs to its question: by default the question is named after
its message and schema, so if a re-run asks something different (it read
different data) the person is asked again. Put what is being decided in the
message ("Delete invoice 1042?", not "Are you sure?"). key names the
question yourself instead; use the same key for the same question on every
run, and a different one whenever what is being asked changes.

Integer properties come back as ints. A question nobody answers within a few
minutes is treated as dismissed: {"action": "cancel"}.

Example:
  answer = tool.ask("Which environment?", {
      "type": "object",
      "properties": {"env": {"type": "string", "enum": ["staging", "prod"]}},
      "required": ["env"],
  })
  if answer is None or answer["action"] != "accept":
      tool.return_error("No environment chosen")
  env = answer["content"]["env"]`,
		},
		"confirm": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 1 {
					return errors.NewError("confirm() takes 1 positional argument (message), got %d", len(args))
				}
				message, errObj := object.CoerceWireString(args[0])
				if errObj != nil {
					return errObj
				}
				if message == "" {
					return errors.NewValueError("confirm(): message must not be empty")
				}
				// Named after the question, so a re-run that asks something
				// else (it read different data) is asked again rather than
				// taking the earlier yes.
				res, exit := elicitNamed(ctx, "confirm-"+shortHash(message), message, confirmSchema)
				if exit != nil {
					return exit
				}
				if res == nil || !res.Accepted() {
					return object.NewBoolean(false)
				}
				approved, _ := res.Content["approve"].(bool)
				return object.NewBoolean(approved)
			},
			HelpText: `confirm(message) - Ask the person to approve an action through the MCP client

Shows message with an Approve box. Returns True only when the person
submitted it with the box ticked; False when they declined, dismissed it or
left it unticked, and when the client cannot be asked (see can_ask()).

As with ask(), the tool may run again from the top to deliver the answer, so
check everything before confirm() and write only after it returns True. The
answer belongs to the message, so name what is being approved in it ("Delete
invoice 1042?", not "Are you sure?"): if a re-run asks about something else,
the person is asked again. The same message may be asked more than once in
one call. A question nobody answers within a few minutes counts as no.

Example:
  if not tool.confirm("Delete the 12 archived invoices?"):
      tool.return_error("Not confirmed, nothing was deleted")
  delete_invoices()`,
		},
		"remember": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 2 {
					return errors.NewError("remember() takes 2 positional arguments (name, value), got %d", len(args))
				}
				name, errObj := object.CoerceWireString(args[0])
				if errObj != nil {
					return errObj
				}
				if name == "" {
					return errors.NewValueError("remember(): name must not be empty")
				}
				value, limitErr := conversion.ToGoLimited(args[1])
				if limitErr != nil {
					return limitErr
				}
				if err := mcplib.Remember(ctx, name, value); err != nil {
					if stderrors.Is(err, mcplib.ErrElicitationUnsupported) {
						return object.NewBoolean(false)
					}
					return errors.NewError("remember(): %v", err)
				}
				return object.NewBoolean(true)
			},
			HelpText: `remember(name, value) - Keep a value for the tool's re-run after a question

When the client answers ask() or confirm() by calling the tool again, the
script runs again from the top. Keep slow or AI work done before the question
with remember() and read it back with recall(), so it is not done twice (and
cannot come out differently the second time).

The value travels to the client with the question and back with the answer,
signed so it cannot be changed: keep it small, and keep only what the person
may see (plain values, lists and dicts). It is a cache, not a record: until
the question state expires, a client can send an earlier round's state back,
and with it earlier kept values. Never let a recalled value authorize
anything on its own (a one-time token, a price the user must not change);
look such things up again before acting on them. Returns False when there will be no
re-run to keep it for; recall() then returns its default, so the work is
simply done each time.

Example:
  plan = tool.recall("plan")
  if plan is None:
      plan = make_plan()          # the slow part
      tool.remember("plan", plan)
  if not tool.confirm("Apply " + plan["summary"] + "?"):
      tool.return_error("Not confirmed")`,
		},
		"recall": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 1 || len(args) > 2 {
					return errors.NewError("recall() takes 1 or 2 positional arguments (name, default), got %d", len(args))
				}
				name, errObj := object.CoerceWireString(args[0])
				if errObj != nil {
					return errObj
				}
				var fallback object.Object = &object.Null{}
				if len(args) == 2 {
					fallback = args[1]
				} else if d := kwargs.Get("default"); d != nil {
					fallback = d
				}
				var raw json.RawMessage
				found, err := mcplib.Recall(ctx, name, &raw)
				if err != nil || !found {
					return fallback
				}
				value, err := decodeKept(raw)
				if err != nil {
					return fallback
				}
				return conversion.FromGo(value)
			},
			HelpText: `recall(name, default=None) - Read back what remember() kept on an earlier run

Returns the value an earlier run of this same tool call kept with
remember(name, value), or default when there is none. See remember().

A kept value travels as JSON, so it comes back as JSON types: a whole number
is an int (also inside lists and dicts), any other number a float, a tuple a
list.`,
		},
	}
}

// askArgs reads ask(message, schema, key="").
func askArgs(fn string, kwargs object.Kwargs, args []object.Object) (message string, schema object.Object, key string, errObj object.Object) {
	get := func(i int, name string) object.Object {
		if i < len(args) {
			return args[i]
		}
		return kwargs.Get(name)
	}
	if len(args) > 3 {
		return "", nil, "", errors.NewError("%s() takes at most 3 positional arguments (message, schema, key), got %d", fn, len(args))
	}
	m := get(0, "message")
	schema = get(1, "schema")
	if m == nil || schema == nil {
		return "", nil, "", errors.NewError("%s() requires a message and a schema", fn)
	}
	message, errObj = object.CoerceWireString(m)
	if errObj != nil {
		return "", nil, "", errObj
	}
	if k := get(2, "key"); k != nil {
		if _, isNull := k.(*object.Null); !isNull {
			if key, errObj = object.CoerceWireString(k); errObj != nil {
				return "", nil, "", errObj
			}
		}
	}
	return message, schema, key, nil
}

// elicitNamed asks under base, or base-2, base-3, ... when the same question
// was already asked in this call: still the same names on every re-run, as
// the script asks in the same order.
func elicitNamed(ctx context.Context, base, message string, schema map[string]any) (*mcplib.ElicitResult, object.Object) {
	for n := 1; ; n++ {
		key := base
		if n > 1 {
			key = fmt.Sprintf("%s-%d", base, n)
		}
		res, exit, used := elicitOnce(ctx, mcplib.ElicitForm{Key: key, Message: message, Schema: schema})
		if !used || n >= 100 {
			return res, exit
		}
	}
}

// shortHash names a question after its text.
func shortHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:8])
}

// decodeKept decodes a value kept with remember. Numbers are read exactly
// (json.Number), and a whole number becomes an int64, so a kept 3 comes back
// as an int rather than the float JSON would otherwise give.
func decodeKept(raw json.RawMessage) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return wholeNumbers(v), nil
}

func wholeNumbers(v any) any {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i
		}
		f, _ := t.Float64()
		return f
	case []any:
		for i, e := range t {
			t[i] = wholeNumbers(e)
		}
		return t
	case map[string]any:
		for k, e := range t {
			t[k] = wholeNumbers(e)
		}
		return t
	}
	return v
}

// typedAnswer returns content with each integer property's value as an
// integer: answers arrive as JSON, where every number is a float.
func typedAnswer(schema, content map[string]any) map[string]any {
	props, _ := schema["properties"].(map[string]any)
	out := make(map[string]any, len(content))
	for name, v := range content {
		p, _ := props[name].(map[string]any)
		if f, ok := v.(float64); ok && p["type"] == "integer" && f == math.Trunc(f) && math.Abs(f) < 1<<53 {
			v = int64(f)
		}
		out[name] = v
	}
	return out
}

// elicitOrStop asks form. It returns the answer, or nil when the client
// cannot be asked; when the question has gone to the client and the tool
// will run again with the answer, it returns exit, which stops the script
// (whatever the tool returns meanwhile is discarded).
func elicitOrStop(ctx context.Context, form mcplib.ElicitForm) (res *mcplib.ElicitResult, exit object.Object) {
	res, exit, used := elicitOnce(ctx, form)
	if used {
		return nil, errors.NewError("the question key %q is already used in this call", form.Key)
	}
	return res, exit
}

// elicitOnce asks form; used reports that its key was already asked in this
// call. Nobody answering in time counts as dismissing the question.
func elicitOnce(ctx context.Context, form mcplib.ElicitForm) (res *mcplib.ElicitResult, exit object.Object, used bool) {
	res, err := mcplib.Elicit(ctx, form)
	switch {
	case err == nil:
		return res, nil, false
	case stderrors.Is(err, mcplib.ErrElicitationUnsupported):
		return nil, nil, false
	case stderrors.Is(err, mcplib.ErrElicitationPending):
		return nil, setResponseAndExit(ctx, "", 0), false
	case stderrors.Is(err, mcplib.ErrElicitationTimeout):
		return &mcplib.ElicitResult{Action: mcplib.ElicitCancel}, nil, false
	case strings.Contains(err.Error(), "is already used in this request"):
		return nil, nil, true
	default:
		return nil, errors.NewError("%v", fmt.Errorf("asking the client failed: %w", err)), false
	}
}
