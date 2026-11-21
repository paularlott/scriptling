package stdlib

import (
	"context"
	"fmt"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

// string.Template: $-substitution. $name and ${name} are placeholders, $$
// is a literal $. A subclass may set a class attribute delimiter.

func isTemplateIdentStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isTemplateIdentChar(c byte) bool {
	return isTemplateIdentStart(c) || c >= '0' && c <= '9'
}

// templatePart is one parsed piece of a template: literal text, or a
// placeholder (name set) spelled raw in the source.
type templatePart struct {
	text, name, raw string
	invalid         bool // a delimiter not followed by a valid placeholder
	line, col       int
}

func parseTemplate(tmpl, delim string) []templatePart {
	var parts []templatePart
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			parts = append(parts, templatePart{text: lit.String()})
			lit.Reset()
		}
	}
	for i := 0; i < len(tmpl); {
		if !strings.HasPrefix(tmpl[i:], delim) {
			lit.WriteByte(tmpl[i])
			i++
			continue
		}
		j := i + len(delim)
		switch {
		case strings.HasPrefix(tmpl[j:], delim):
			lit.WriteString(delim)
			i = j + len(delim)
			continue
		case j < len(tmpl) && isTemplateIdentStart(tmpl[j]):
			k := j + 1
			for k < len(tmpl) && isTemplateIdentChar(tmpl[k]) {
				k++
			}
			flush()
			parts = append(parts, templatePart{name: tmpl[j:k], raw: tmpl[i:k]})
			i = k
			continue
		case j+1 < len(tmpl) && tmpl[j] == '{' && isTemplateIdentStart(tmpl[j+1]):
			k := j + 2
			for k < len(tmpl) && isTemplateIdentChar(tmpl[k]) {
				k++
			}
			if k < len(tmpl) && tmpl[k] == '}' {
				flush()
				parts = append(parts, templatePart{name: tmpl[j+1 : k], raw: tmpl[i : k+1]})
				i = k + 1
				continue
			}
		}
		// Invalid placeholder: Python reports its 1-based line and column.
		flush()
		line := strings.Count(tmpl[:i], "\n") + 1
		col := i - strings.LastIndexByte(tmpl[:i], '\n')
		parts = append(parts, templatePart{raw: delim, invalid: true, line: line, col: col})
		i = j
	}
	flush()
	return parts
}

// templateValueString is str(v) for a substituted value.
func templateValueString(ctx context.Context, v object.Object) (string, object.Object) {
	switch x := v.(type) {
	case *object.String:
		return x.StringValue(), nil
	case *object.Instance:
		if m, ok := x.Class.LookupMember("__str__"); ok {
			if eval := evaliface.FromContext(ctx); eval != nil {
				r := eval.CallObjectFunction(ctx, m, []object.Object{x}, nil, nil)
				if object.IsPropagating(r) {
					return "", r
				}
				if s, ok := r.(*object.String); ok {
					return s.StringValue(), nil
				}
			}
		}
	}
	return v.Inspect(), nil
}

// templateSubstitute implements substitute (safe=false) and safe_substitute.
func templateSubstitute(safe bool) func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	return func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		self, err := selfInstance(args, "substitute")
		if err != nil {
			return err
		}
		if len(args) > 2 {
			return errors.NewTypeErrorTagged("Too many positional arguments")
		}
		var mapping *object.Dict
		if len(args) == 2 {
			d, ok := args[1].(*object.Dict)
			if !ok {
				return errors.NewTypeErrorTagged("substitute() mapping must be a dict, not %s", object.PythonTypeName(args[1]))
			}
			mapping = d
		}
		lookup := func(name string) (object.Object, bool) {
			if v, ok := kwargs.Lookup(name); ok {
				return v, true
			}
			if mapping != nil {
				if p, ok := mapping.Lookup(object.DictStringKey(name)); ok {
					return p.Value, true
				}
			}
			return nil, false
		}
		tmpl, _ := self.Field("template").(*object.String)
		if tmpl == nil {
			return errors.NewTypeErrorTagged("Template has no template string")
		}
		var b strings.Builder
		for _, p := range parseTemplate(tmpl.StringValue(), templateDelimiter(self)) {
			switch {
			case p.invalid:
				if !safe {
					return errors.NewValueError("Invalid placeholder in string: line %d, col %d", p.line, p.col)
				}
				b.WriteString(p.raw)
			case p.name != "":
				v, ok := lookup(p.name)
				if !ok {
					if !safe {
						return &object.Exception{Message: object.ReprString(p.name), ExceptionType: object.ExceptionTypeKeyError, Raised: true}
					}
					b.WriteString(p.raw)
					continue
				}
				s, serr := templateValueString(ctx, v)
				if serr != nil {
					return serr
				}
				b.WriteString(s)
			default:
				b.WriteString(p.text)
			}
		}
		return object.NewString(b.String())
	}
}

// templateDelimiter is the delimiter class attribute ("$" by default).
func templateDelimiter(self *object.Instance) string {
	if d, ok := self.Class.LookupMember("delimiter"); ok {
		if s, ok := d.(*object.String); ok && s.StringValue() != "" {
			return s.StringValue()
		}
	}
	return "$"
}

// TemplateClass is string.Template.
var TemplateClass = &object.Class{
	Name: "Template",
	Methods: map[string]object.Object{
		"delimiter": object.NewString("$"),
		"__init__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("Template", kwargs); err != nil {
					return err
				}
				self, err := selfInstance(args, "__init__")
				if err != nil {
					return err
				}
				if len(args) != 2 {
					return errors.NewTypeErrorTagged("Template.__init__() takes 1 argument (%d given)", len(args)-1)
				}
				s, ok := args[1].(*object.String)
				if !ok {
					return errors.NewTypeErrorTagged("Template() argument must be str, not %s", object.PythonTypeName(args[1]))
				}
				self.SetField("template", s)
				return &object.Null{}
			},
			HelpText: "Template(template) - A $-substitution template",
		},
		"substitute": &object.Builtin{
			Fn: templateSubstitute(false),
			HelpText: `substitute(mapping={}, /, **kwds) - Fill the placeholders

$name and ${name} are replaced from mapping and keyword arguments (keywords
win); $$ is a literal $. A missing name raises KeyError, a malformed
placeholder ValueError.`,
		},
		"safe_substitute": &object.Builtin{
			Fn: templateSubstitute(true),
			HelpText: `safe_substitute(mapping={}, /, **kwds) - Fill the placeholders that have values

Like substitute, but missing names and malformed placeholders are left as
they are instead of raising.`,
		},
		"get_identifiers": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("get_identifiers", kwargs); err != nil {
					return err
				}
				self, err := selfInstance(args, "get_identifiers")
				if err != nil {
					return err
				}
				tmpl, _ := self.Field("template").(*object.String)
				if tmpl == nil {
					return &object.List{}
				}
				seen := map[string]bool{}
				var out []object.Object
				for _, p := range parseTemplate(tmpl.StringValue(), templateDelimiter(self)) {
					if p.name != "" && !seen[p.name] {
						seen[p.name] = true
						out = append(out, object.NewString(p.name))
					}
				}
				return &object.List{Elements: out}
			},
			HelpText: "get_identifiers() - The placeholder names, in order of first appearance",
		},
		"is_valid": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("is_valid", kwargs); err != nil {
					return err
				}
				self, err := selfInstance(args, "is_valid")
				if err != nil {
					return err
				}
				tmpl, _ := self.Field("template").(*object.String)
				if tmpl == nil {
					return object.NewBoolean(false)
				}
				for _, p := range parseTemplate(tmpl.StringValue(), templateDelimiter(self)) {
					if p.invalid {
						return object.NewBoolean(false)
					}
				}
				return object.NewBoolean(true)
			},
			HelpText: "is_valid() - False when the template has a malformed placeholder",
		},
		"__repr__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("__repr__", kwargs); err != nil {
					return err
				}
				self, err := selfInstance(args, "__repr__")
				if err != nil {
					return err
				}
				return object.NewString(fmt.Sprintf("<string.Template object at %p>", self))
			},
			HelpText: "__repr__() - <string.Template object at ...>",
		},
	},
}
