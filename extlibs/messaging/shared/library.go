package shared

import (
	"context"
	"fmt"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)


// ScriptSender is the interface the shared Scriptling library calls on the wrapped client instance.
// It mirrors Sender but operates on the Scriptling object.Instance that scripts hold.
type ScriptSender interface {
	Sender
	// Bot framework accessors — called by the shared library functions
	BotCommand(name, helpText string, h Handler)
	BotOnCallback(prefix string, h Handler)
	BotOnMessage(h Handler)
	BotOnFile(h Handler)
	BotAuth(h Handler)
	BotRun(ctx context.Context) error
	BotCapabilities() []string
}

// richMessageFromDict converts a Scriptling dict to a *RichMessage.
func richMessageFromDict(d *object.Dict) *RichMessage {
	get := func(key string) string {
		if p, ok := d.GetByString(key); ok {
			if s, err := p.Value.AsString(); err == nil {
				return s
			}
		}
		return ""
	}
	return &RichMessage{
		Title: get("title"),
		Body:  get("body"),
		Color: get("color"),
		Image: get("image"),
		URL:   get("url"),
	}
}

// keyboardFromObject converts a Scriptling list-of-lists to a *Keyboard.
func keyboardFromObject(obj object.Object) *Keyboard {
	rows, ok := obj.(*object.List)
	if !ok {
		return nil
	}
	kb := make(Keyboard, 0, len(rows.Elements))
	for _, rowObj := range rows.Elements {
		row, ok := rowObj.(*object.List)
		if !ok {
			continue
		}
		var btns []KeyboardButton
		for _, btnObj := range row.Elements {
			d, ok := btnObj.(*object.Dict)
			if !ok {
				continue
			}
			get := func(k string) string {
				if p, ok := d.GetByString(k); ok {
					if s, err := p.Value.AsString(); err == nil {
						return s
					}
				}
				return ""
			}
			btns = append(btns, KeyboardButton{
				Text: get("text"),
				Data: get("data"),
				URL:  get("url"),
			})
		}
		kb = append(kb, btns)
	}
	return &kb
}

// envFromContext extracts the Scriptling environment from a Go context.
func envFromContext(ctx context.Context) *object.Environment {
	if env, ok := ctx.Value("scriptling-env").(*object.Environment); ok {
		return env
	}
	return object.NewEnvironment()
}

// wrapScriptHandler wraps a Scriptling callable as a shared.Handler.
// The handler is called as fn(ctx_dict) where ctx_dict exposes the update fields.
// Returns non-nil error only for script errors; auth handlers return error to deny.
func wrapScriptHandler(eval evaliface.Evaluator, fn object.Object, inst *object.Instance, env *object.Environment, isAuth bool) Handler {
	return func(goCtx context.Context, c *Ctx) error {
		d := BuildCtxDict(c)
		result := eval.CallObjectFunction(goCtx, fn, []object.Object{d}, nil, env)
		if result == nil {
			return nil
		}
		if errObj, isErr := result.(*object.Error); isErr {
			if isAuth {
				return errDenied
			}
			return fmt.Errorf("handler error: %s", errObj.Message)
		}
		if isAuth {
			// auth handler: False return → deny
			if b, ok := result.(*object.Boolean); ok && !b.Value {
				return errDenied
			}
		}
		return nil
	}
}

// errDenied is a sentinel returned by auth handlers to deny access.
var errDenied = fmt.Errorf("denied")

// BuildCtxDict builds the Scriptling dict passed to handlers.
func BuildCtxDict(c *Ctx) *object.Dict {
	u := c.Update
	d := &object.Dict{Pairs: make(map[string]object.DictPair)}
	d.SetByString("dest", &object.String{Value: u.Dest})
	d.SetByString("message_id", &object.String{Value: u.MessageID})
	d.SetByString("text", &object.String{Value: u.Text})
	d.SetByString("command", &object.String{Value: u.Command})
	d.SetByString("is_callback", object.NewBoolean(u.IsCallback))
	d.SetByString("callback_id", &object.String{Value: u.CallbackID})
	d.SetByString("callback_token", &object.String{Value: u.CallbackToken})
	d.SetByString("callback_data", &object.String{Value: u.CallbackData})

	// args list
	args := make([]object.Object, len(u.Args))
	for i, a := range u.Args {
		args[i] = &object.String{Value: a}
	}
	d.SetByString("args", &object.List{Elements: args})

	// user dict
	user := &object.Dict{Pairs: make(map[string]object.DictPair)}
	user.SetByString("id", &object.String{Value: u.UserID})
	user.SetByString("name", &object.String{Value: u.UserName})
	user.SetByString("platform", &object.String{Value: c.Sender.Platform()})
	d.SetByString("user", user)

	if u.File != nil {
		fd := &object.Dict{Pairs: make(map[string]object.DictPair)}
		fd.SetByString("id", &object.String{Value: u.File.ID})
		fd.SetByString("name", &object.String{Value: u.File.Name})
		fd.SetByString("mime", &object.String{Value: u.File.MimeType})
		fd.SetByString("size", object.NewInteger(u.File.Size))
		fd.SetByString("url", &object.String{Value: u.File.URL})
		d.SetByString("file", fd)
	} else {
		d.SetByString("file", &object.Null{})
	}

	// Inject reply helpers so scripts can do ctx.reply(...) / ctx.typing() etc.
	d.SetByString("reply", &object.Builtin{
		Fn: func(goCtx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) == 0 {
				return errors.NewError("reply: expected 1 argument")
			}
			if dict, ok := args[0].(*object.Dict); ok {
				if err := c.ReplyRich(goCtx, richMessageFromDict(dict)); err != nil {
					return errors.NewError("reply: %s", err.Error())
				}
				return &object.Null{}
			}
			text, _ := args[0].AsString()
			opts := &SendOptions{ParseMode: kwargs.MustGetString("parse_mode", "")}
			if kb := kwargs.Get("keyboard"); kb != nil {
				opts.Keyboard = keyboardFromObject(kb)
			}
			if err := c.Reply(goCtx, text, opts); err != nil {
				return errors.NewError("reply: %s", err.Error())
			}
			return &object.Null{}
		},
		HelpText: `ctx.reply(text_or_dict, parse_mode="", keyboard=None)`,
	})
	d.SetByString("typing", &object.Builtin{
		Fn: func(goCtx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := c.Typing(goCtx); err != nil {
				return errors.NewError("typing: %s", err.Error())
			}
			return &object.Null{}
		},
		HelpText: `ctx.typing()`,
	})
	d.SetByString("answer", &object.Builtin{
		Fn: func(goCtx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			text := ""
			if len(args) > 0 {
				text, _ = args[0].AsString()
			}
			if err := c.Answer(goCtx, text); err != nil {
				return errors.NewError("answer: %s", err.Error())
			}
			return &object.Null{}
		},
		HelpText: `ctx.answer(text="") - acknowledge a callback/button press`,
	})
	d.SetByString("download", &object.Builtin{
		Fn: func(goCtx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			data, err := c.Download(goCtx)
			if err != nil {
				return errors.NewError("download: %s", err.Error())
			}
			if data == nil {
				return &object.Null{}
			}
			return &object.String{Value: EncodeBase64(data)}
		},
		HelpText: `ctx.download() - download the file/photo in this update, returns base64 string`,
	})

	d.SetByString("capabilities", &object.Builtin{
		Fn: func(goCtx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			caps := c.Sender.Capabilities()
			elems := make([]object.Object, len(caps))
			for i, s := range caps {
				elems[i] = &object.String{Value: s}
			}
			return &object.List{Elements: elems}
		},
		HelpText: `ctx.capabilities() - Return list of capability strings for this platform`,
	})
	d.SetByString("has_capability", &object.Builtin{
		Fn: func(goCtx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) == 0 {
				return errors.NewError("has_capability: expected 1 argument")
			}
			name, _ := args[0].AsString()
			for _, cap := range c.Sender.Capabilities() {
				if cap == name {
					return object.NewBoolean(true)
				}
			}
			return object.NewBoolean(false)
		},
		HelpText: `ctx.has_capability(name) - Return True if the platform supports the named capability`,
	})

	return d
}

// clientFrom extracts the ScriptSender from args[0] (the instance self).
func ClientFrom(nativeKey string, args []object.Object) (ScriptSender, *object.Instance, bool) {
	inst, ok := args[0].(*object.Instance)
	if !ok {
		return nil, nil, false
	}
	s, ok := inst.Fields[nativeKey].(ScriptSender)
	if !ok {
		return nil, nil, false
	}
	return s, inst, true
}

// BindToInstance injects all shared builtins as instance fields so scripts can call
// client.command(...), client.run(), etc. Each builtin is wrapped to prepend the
// instance as args[0], matching the module-level calling convention.
func BindToInstance(inst *object.Instance, builtins map[string]*object.Builtin) {
	for name, b := range builtins {
		fn := b // capture
		inst.Fields[name] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				newArgs := make([]object.Object, len(args)+1)
				newArgs[0] = inst
				copy(newArgs[1:], args)
				return fn.Fn(ctx, kwargs, newArgs...)
			},
			HelpText: fn.HelpText,
		}
	}
}

// SharedBuiltins returns the map of Scriptling builtins that are identical across all platforms.
// nativeKey is the field name used to store the ScriptSender in the Instance.Fields map.
func SharedBuiltins(nativeKey string) map[string]*object.Builtin {
	return map[string]*object.Builtin{

		"capabilities": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				s, _, ok := ClientFrom(nativeKey, args)
				if !ok {
					return errors.NewError("capabilities: invalid client")
				}
				caps := s.BotCapabilities()
				elems := make([]object.Object, len(caps))
				for i, c := range caps {
					elems[i] = &object.String{Value: c}
				}
				return &object.List{Elements: elems}
			},
			HelpText: `capabilities(client) - Return list of capability strings for this platform`,
		},

		"command": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.RangeArgs(args, 3, 4); err != nil {
					return err
				}
				s, inst, ok := ClientFrom(nativeKey, args)
				if !ok {
					return errors.NewError("command: invalid client")
				}
				name, _ := args[1].AsString()
				helpText := ""
				var fnArg object.Object
				if len(args) == 4 {
					helpText, _ = args[2].AsString()
					fnArg = args[3]
				} else {
					fnArg = args[2]
				}
				eval := evaliface.FromContext(ctx)
				if eval == nil {
					return errors.NewError("command: no evaluator in context")
				}
				env := envFromContext(ctx)
				s.BotCommand(name, helpText, wrapScriptHandler(eval, fnArg, inst, env, false))
				return &object.Null{}
			},
			HelpText: `command(client, name, handler) or command(client, name, help_text, handler) - Register a command handler`,
		},

		"on_callback": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.RangeArgs(args, 2, 3); err != nil {
					return err
				}
				s, inst, ok := ClientFrom(nativeKey, args)
				if !ok {
					return errors.NewError("on_callback: invalid client")
				}
				prefix := ""
				var fnArg object.Object
				if len(args) == 3 {
					prefix, _ = args[1].AsString()
					fnArg = args[2]
				} else {
					fnArg = args[1]
				}
				eval := evaliface.FromContext(ctx)
				if eval == nil {
					return errors.NewError("on_callback: no evaluator in context")
				}
				env := envFromContext(ctx)
				s.BotOnCallback(prefix, wrapScriptHandler(eval, fnArg, inst, env, false))
				return &object.Null{}
			},
			HelpText: `on_callback(client, handler) or on_callback(client, prefix, handler) - Register a callback/button handler`,
		},

		"on_message": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				s, inst, ok := ClientFrom(nativeKey, args)
				if !ok {
					return errors.NewError("on_message: invalid client")
				}
				eval := evaliface.FromContext(ctx)
				if eval == nil {
					return errors.NewError("on_message: no evaluator in context")
				}
				env := envFromContext(ctx)
				s.BotOnMessage(wrapScriptHandler(eval, args[1], inst, env, false))
				return &object.Null{}
			},
			HelpText: `on_message(client, handler) - Register default message handler`,
		},

		"on_file": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				s, inst, ok := ClientFrom(nativeKey, args)
				if !ok {
					return errors.NewError("on_file: invalid client")
				}
				eval := evaliface.FromContext(ctx)
				if eval == nil {
					return errors.NewError("on_file: no evaluator in context")
				}
				env := envFromContext(ctx)
				s.BotOnFile(wrapScriptHandler(eval, args[1], inst, env, false))
				return &object.Null{}
			},
			HelpText: `on_file(client, handler) - Register file attachment handler`,
		},

		"auth": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				s, inst, ok := ClientFrom(nativeKey, args)
				if !ok {
					return errors.NewError("auth: invalid client")
				}
				eval := evaliface.FromContext(ctx)
				if eval == nil {
					return errors.NewError("auth: no evaluator in context")
				}
				env := envFromContext(ctx)
				s.BotAuth(wrapScriptHandler(eval, args[1], inst, env, true))
				return &object.Null{}
			},
			HelpText: `auth(client, handler) - Register auth handler; return True to allow, False to deny`,
		},

		"run": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				s, _, ok := ClientFrom(nativeKey, args)
				if !ok {
					return errors.NewError("run: invalid client")
				}
				if err := s.BotRun(ctx); err != nil {
					return errors.NewError("run: %s", err.Error())
				}
				return &object.Null{}
			},
			HelpText: `run(client) - Start the bot event loop (blocks until stopped)`,
		},

		"send_message": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.MinArgs(args, 3); err != nil {
					return err
				}
				s, _, ok := ClientFrom(nativeKey, args)
				if !ok {
					return errors.NewError("send_message: invalid client")
				}
				dest, errObj := args[1].AsString()
				if errObj != nil {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}
				// Accept string or dict (rich message)
				if d, ok := args[2].(*object.Dict); ok {
					if err := s.SendRichMessage(ctx, dest, richMessageFromDict(d)); err != nil {
						return errors.NewError("send_message: %s", err.Error())
					}
					return &object.Null{}
				}
				text, errObj := args[2].AsString()
				if errObj != nil {
					return errors.NewTypeError("STRING or DICT", args[2].Type().String())
				}
				opts := &SendOptions{
					ParseMode: kwargs.MustGetString("parse_mode", ""),
				}
				if kb := kwargs.Get("keyboard"); kb != nil {
					opts.Keyboard = keyboardFromObject(kb)
				}
				if err := s.SendMessage(ctx, dest, text, opts); err != nil {
					return errors.NewError("send_message: %s", err.Error())
				}
				return &object.Null{}
			},
			HelpText: `send_message(client, dest, text_or_dict, parse_mode="", keyboard=None) - Send a text or rich message`,
		},

		"send_rich_message": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 3); err != nil {
					return err
				}
				s, _, ok := ClientFrom(nativeKey, args)
				if !ok {
					return errors.NewError("send_rich_message: invalid client")
				}
				dest, errObj := args[1].AsString()
				if errObj != nil {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}
				d, ok := args[2].(*object.Dict)
				if !ok {
					return errors.NewTypeError("DICT", args[2].Type().String())
				}
				if err := s.SendRichMessage(ctx, dest, richMessageFromDict(d)); err != nil {
					return errors.NewError("send_rich_message: %s", err.Error())
				}
				return &object.Null{}
			},
			HelpText: `send_rich_message(client, dest, msg) - Send a rich message dict with title, body, color, image, url keys`,
		},

		"edit_message": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 4); err != nil {
					return err
				}
				s, _, ok := ClientFrom(nativeKey, args)
				if !ok {
					return errors.NewError("edit_message: invalid client")
				}
				dest, _ := args[1].AsString()
				msgID, _ := args[2].AsString()
				text, _ := args[3].AsString()
				if err := s.EditMessage(ctx, dest, msgID, text); err != nil {
					return errors.NewError("edit_message: %s", err.Error())
				}
				return &object.Null{}
			},
			HelpText: `edit_message(client, dest, message_id, text) - Edit a sent message`,
		},

		"delete_message": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 3); err != nil {
					return err
				}
				s, _, ok := ClientFrom(nativeKey, args)
				if !ok {
					return errors.NewError("delete_message: invalid client")
				}
				dest, _ := args[1].AsString()
				msgID, _ := args[2].AsString()
				if err := s.DeleteMessage(ctx, dest, msgID); err != nil {
					return errors.NewError("delete_message: %s", err.Error())
				}
				return &object.Null{}
			},
			HelpText: `delete_message(client, dest, message_id) - Delete a message`,
		},

		"send_file": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.MinArgs(args, 3); err != nil {
					return err
				}
				s, _, ok := ClientFrom(nativeKey, args)
				if !ok {
					return errors.NewError("send_file: invalid client")
				}
				dest, _ := args[1].AsString()
				source, _ := args[2].AsString()
				fileName := kwargs.MustGetString("filename", "")
				caption := kwargs.MustGetString("caption", "")
				isB64 := kwargs.MustGetBool("base64", false)
				if err := s.SendFile(ctx, dest, source, fileName, caption, isB64); err != nil {
					return errors.NewError("send_file: %s", err.Error())
				}
				return &object.Null{}
			},
			HelpText: `send_file(client, dest, source, filename="", caption="", base64=False) - Send a file`,
		},

		"typing": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				s, _, ok := ClientFrom(nativeKey, args)
				if !ok {
					return errors.NewError("typing: invalid client")
				}
				dest, _ := args[1].AsString()
				if err := s.SendTyping(ctx, dest); err != nil {
					return errors.NewError("typing: %s", err.Error())
				}
				return &object.Null{}
			},
			HelpText: `typing(client, dest) - Send typing indicator`,
		},

		"answer_callback": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.RangeArgs(args, 2, 4); err != nil {
					return err
				}
				s, _, ok := ClientFrom(nativeKey, args)
				if !ok {
					return errors.NewError("answer_callback: invalid client")
				}
				id, _ := args[1].AsString()
				token := ""
				text := ""
				// answer_callback(client, id, text)           — Telegram style
				// answer_callback(client, id, token, text)    — Discord style
				if len(args) == 3 {
					text, _ = args[2].AsString()
				} else if len(args) == 4 {
					token, _ = args[2].AsString()
					text, _ = args[3].AsString()
				}
				if err := s.AckCallback(ctx, id, token, text); err != nil {
					return errors.NewError("answer_callback: %s", err.Error())
				}
				return &object.Null{}
			},
			HelpText: `answer_callback(client, id, text="") or answer_callback(client, id, token, text="") - Acknowledge a button press`,
		},

		"download": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				s, _, ok := ClientFrom(nativeKey, args)
				if !ok {
					return errors.NewError("download: invalid client")
				}
				ref, _ := args[1].AsString()
				data, err := s.Download(ctx, ref)
				if err != nil {
					return errors.NewError("download: %s", err.Error())
				}
				return &object.String{Value: EncodeBase64(data)}
			},
			HelpText: `download(client, ref) - Download a file by ID or URL, returns base64 string`,
		},
	}
}

// KeyboardBuiltin is a free-standing (non-client-bound) builtin that passes rows through.
// Added directly to each platform library so it is available as telegram.keyboard(...) /
// discord.keyboard(...) without being bound to a client instance.
var KeyboardBuiltin = &object.Builtin{
	Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		return args[0]
	},
	HelpText: `keyboard(rows) - Build a platform-agnostic button keyboard; rows is a list of lists of {text, data} or {text, url} dicts`,
}
