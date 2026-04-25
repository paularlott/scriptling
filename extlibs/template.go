package extlibs

import (
	"bytes"
	"context"
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

func renderTemplateSet(pt *parsedTemplateSet, name string, data interface{}) (string, error) {
	var buf bytes.Buffer
	var err error
	if pt.html != nil {
		if name == "" {
			err = pt.html.Execute(&buf, data)
		} else {
			err = pt.html.ExecuteTemplate(&buf, name, data)
		}
	} else {
		if name == "" {
			err = pt.text.Execute(&buf, data)
		} else {
			err = pt.text.ExecuteTemplate(&buf, name, data)
		}
	}
	return buf.String(), err
}

func newSetInstance(pt *parsedTemplateSet) *object.Instance {
	return &object.Instance{
		Class:      TemplateSetClass,
		Fields:     map[string]object.Object{},
		NativeData: pt,
	}
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

				result, execErr := renderTemplateSet(pt, name, conversion.ToGo(data))
				if execErr != nil {
					return errors.NewError("template render error: %s", execErr.Error())
				}
				return &object.String{Value: result}
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
			return newSetInstance(&parsedTemplateSet{html: htmltemplate.New("")})
		},
		HelpText: `Set() - Create a new HTML template set (uses html/template with auto-escaping)

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
  print(tmpl.render("page", {"Title": "Home", "Body": "Welcome"}))`,
	},
}, map[string]object.Object{}, "Go html/template rendering with automatic HTML escaping")

// TemplateTextLibrary provides text/template rendering with no escaping
var TemplateTextLibrary = object.NewLibrary(TemplateTextLibraryName, map[string]*object.Builtin{
	"Set": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 0); err != nil {
				return err
			}
			return newSetInstance(&parsedTemplateSet{text: texttemplate.New("")})
		},
		HelpText: `Set() - Create a new text template set (uses text/template, no HTML escaping)

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
  print(tmpl.render("email", {"Name": "Alice"}))`,
	},
}, map[string]object.Object{}, "Go text/template rendering with no escaping")

func RegisterTemplateHTMLLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(TemplateHTMLLibrary)
}

func RegisterTemplateTextLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(TemplateTextLibrary)
}
