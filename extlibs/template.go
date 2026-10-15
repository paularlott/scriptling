package extlibs

import (
	"bytes"
	"context"
	stderrors "errors"
	htmltemplate "html/template"
	texttemplate "text/template"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

type parsedTemplateSet struct {
	html *htmltemplate.Template
	text *texttemplate.Template
}

func getTemplateSet(self *object.Instance) *parsedTemplateSet {
	pt, _ := self.NativeData.(*parsedTemplateSet)
	return pt
}

func renderTemplateSet(ctx context.Context, pt *parsedTemplateSet, name string, data interface{}) (string, error) {
	out := &renderWriter{ctx: ctx, limit: object.MaxValueBytes()}
	var err error
	if pt.html != nil {
		if name == "" {
			err = pt.html.Execute(out, data)
		} else {
			err = pt.html.ExecuteTemplate(out, name, data)
		}
	} else {
		if name == "" {
			err = pt.text.Execute(out, data)
		} else {
			err = pt.text.ExecuteTemplate(out, name, data)
		}
	}
	return out.buf.String(), err
}

// errRenderTooLarge stops a render whose output passes the value limit.
var errRenderTooLarge = stderrors.New("rendered output exceeds the value limit")

// errRenderNoRoom stops a render whose output no longer fits in the memory
// limit.
var errRenderNoRoom = stderrors.New("rendered output exceeds the memory limit")

// renderWriter collects template output, stopping the render (Go templates
// stop at the first write error) when the output passes the interpreter's
// value limit or the call is cancelled.
type renderWriter struct {
	ctx   context.Context
	limit int64
	buf   bytes.Buffer
}

func (w *renderWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	size := int64(w.buf.Len() + len(p))
	if w.limit > 0 && size > w.limit {
		return 0, errRenderTooLarge
	}
	growing := size > int64(w.buf.Cap())
	if growing && object.CheckRoom("rendered template", object.GrowthRoom(w.buf.Cap(), int(size))) != nil {
		return 0, errRenderNoRoom
	}
	n, err := w.buf.Write(p)
	if growing {
		// Charge the new backing array; the old one is credited once freed.
		if b := w.buf.Bytes(); cap(b) > 0 {
			object.TrackAlloc(&b[:1][0], int64(cap(b)))
		}
	}
	return n, err
}

func newSetInstance(pt *parsedTemplateSet) *object.Instance {
	return object.NewInstanceWithData(TemplateSetClass, nil, pt)
}

// TemplateSetClass is the class for Set objects
var TemplateSetClass = &object.Class{
	Name: "Set",
	Methods: map[string]object.Object{
		"add": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				self, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewError("add() called on non-Set object")
				}
				src, err := args[1].AsString()
				if err != nil {
					return err
				}
				pt := getTemplateSet(self)
				if pt == nil {
					return errors.NewError("Set not initialised")
				}
				if pt.html != nil {
					if _, parseErr := pt.html.Parse(src); parseErr != nil {
						return errors.NewError("template parse error: %s", parseErr.Error())
					}
				} else {
					if _, parseErr := pt.text.Parse(src); parseErr != nil {
						return errors.NewError("template parse error: %s", parseErr.Error())
					}
				}
				return &object.Null{}
			},
			HelpText: `add(source) - Add a template source to the set

Parameters:
  source (string): Template source, may contain {{define "name"}}...{{end}} blocks

Example:
  tmpl = html.Set()
  tmpl.add('{{define "header"}}<h1>{{.Title}}</h1>{{end}}')
  tmpl.add('{{define "page"}}{{template "header" .}}<p>{{.Body}}</p>{{end}}')`,
		},
		"render": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.RangeArgs(args, 1, 3); err != nil {
					return err
				}
				self, ok := args[0].(*object.Instance)
				if !ok {
					return errors.NewError("render() called on non-Set object")
				}

				// render(data) or render(name, data)
				var name string
				var data object.Object = &object.Null{}
				switch len(args) {
				case 2:
					data = args[1]
				case 3:
					n, err := args[1].AsString()
					if err != nil {
						return err
					}
					name = n
					data = args[2]
				}

				pt := getTemplateSet(self)
				if pt == nil {
					return errors.NewError("Set not initialised")
				}

				// Shared script values convert once per reference, so the
				// data is converted under the interpreter's limits.
				goData, limitErr := conversion.ToGoLimited(data)
				if limitErr != nil {
					return limitErr
				}
				result, execErr := renderTemplateSet(ctx, pt, name, goData)
				if stderrors.Is(execErr, errRenderNoRoom) {
					return object.NewMemoryError("%s: no room for the rendered template", object.ErrMemoryLimitExceeded)
				}
				if stderrors.Is(execErr, errRenderTooLarge) {
					return object.NewMemoryError("rendered template too large: the limit is %d bytes", object.MaxValueBytes())
				}
				if execErr != nil {
					return errors.NewError("template render error: %s", execErr.Error())
				}
				return object.NewString(result)
			},
			HelpText: `render(data) or render(name, data) - Render a template from the set

Parameters:
  name (string, optional): Name of the template to render (from {{define "name"}})
  data (dict): Template data

Returns:
  string: Rendered output

Example:
  # Anonymous / single template
  tmpl.render({"Name": "Alice"})

  # Named template
  tmpl.render("page", {"Title": "Home", "Body": "Welcome"})`,
		},
	},
}

// TemplateHTMLLibrary provides html/template rendering with automatic HTML escaping
var TemplateHTMLLibrary = object.NewLibrary(TemplateHTMLLibraryName, map[string]*object.Builtin{
	"Set": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 0); err != nil {
				return err
			}
			left, errObj := kwargs.GetString("left", "")
			if errObj != nil {
				return errObj
			}
			right, errObj := kwargs.GetString("right", "")
			if errObj != nil {
				return errObj
			}
			t := htmltemplate.New("").Delims(left, right)
			return newSetInstance(&parsedTemplateSet{html: t})
		},
		HelpText: `Set(left="", right="") - Create a new HTML template set (uses html/template with auto-escaping)

Parameters:
  left (str, optional): Left action delimiter, default "{{"
  right (str, optional): Right action delimiter, default "}}"

Returns:
  Set: A template set with add(source) and render([name,] data) methods

Example:
  import scriptling.template.html as html

  # Simple template
  tmpl = html.Set()
  tmpl.add("Hello, {{.Name}}!")
  print(tmpl.render({"Name": "Alice"}))

  # With partials
  tmpl = html.Set()
  tmpl.add('{{define "header"}}<h1>{{.Title}}</h1>{{end}}')
  tmpl.add('{{define "page"}}{{template "header" .}}<p>{{.Body}}</p>{{end}}')
  print(tmpl.render("page", {"Title": "Home", "Body": "Welcome"}))

  # Custom delimiters, e.g. {% %} to avoid clashing with literal {{ }}
  tmpl = html.Set(left="{%", right="%}")
  tmpl.add("<p>{%.Name%}</p>")`,
	},
}, map[string]object.Object{}, "Go html/template rendering with automatic HTML escaping")

// TemplateTextLibrary provides text/template rendering with no escaping
var TemplateTextLibrary = object.NewLibrary(TemplateTextLibraryName, map[string]*object.Builtin{
	"Set": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 0); err != nil {
				return err
			}
			left, errObj := kwargs.GetString("left", "")
			if errObj != nil {
				return errObj
			}
			right, errObj := kwargs.GetString("right", "")
			if errObj != nil {
				return errObj
			}
			t := texttemplate.New("").Delims(left, right)
			return newSetInstance(&parsedTemplateSet{text: t})
		},
		HelpText: `Set(left="", right="") - Create a new text template set (uses text/template, no HTML escaping)

Parameters:
  left (str, optional): Left action delimiter, default "{{"
  right (str, optional): Right action delimiter, default "}}"

Returns:
  Set: A template set with add(source) and render([name,] data) methods

Example:
  import scriptling.template.text as text

  # Simple template
  tmpl = text.Set()
  tmpl.add("Hello, {{.Name}}!")
  print(tmpl.render({"Name": "Alice"}))

  # With partials
  tmpl = text.Set()
  tmpl.add('{{define "greeting"}}Hello, {{.Name}}!{{end}}')
  tmpl.add('{{define "email"}}{{template "greeting" .}}\n\nYour order is ready.{{end}}')
  print(tmpl.render("email", {"Name": "Alice"}))

  # Custom delimiters, e.g. {% %} to avoid clashing with literal {{ }}
  tmpl = text.Set(left="{%", right="%}")
  tmpl.add("Hello, {%.Name%}!")`,
	},
}, map[string]object.Object{}, "Go text/template rendering with no escaping")

func RegisterTemplateHTMLLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(TemplateHTMLLibrary)
}

func RegisterTemplateTextLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(TemplateTextLibrary)
}
