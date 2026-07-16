package extlibs

import (
	"context"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// --- helpers ---------------------------------------------------------------

// dict builds a *object.Dict for render data from string key → value pairs.
func dict(vals ...any) *object.Dict {
	m := map[string]object.Object{}
	for i := 0; i+1 < len(vals); i += 2 {
		key := vals[i].(string)
		switch v := vals[i+1].(type) {
		case string:
			m[key] = object.NewString(v)
		case int64:
			m[key] = object.NewInteger(v)
		case int:
			m[key] = object.NewInteger(int64(v))
		default:
			m[key] = v.(object.Object)
		}
	}
	return object.NewStringDict(m)
}

// kw builds a Kwargs map from string keys to Object values.
func kw(vals ...any) object.Kwargs {
	m := map[string]object.Object{}
	for i := 0; i+1 < len(vals); i += 2 {
		key := vals[i].(string)
		switch v := vals[i+1].(type) {
		case string:
			m[key] = object.NewString(v)
		case int64:
			m[key] = object.NewInteger(v)
		case int:
			m[key] = object.NewInteger(int64(v))
		default:
			m[key] = v.(object.Object)
		}
	}
	return object.NewKwargs(m)
}

// methodFn fetches a Set method's function, type-asserting it to *object.Builtin.
func methodFn(t *testing.T, name string) object.BuiltinFunction {
	t.Helper()
	obj := TemplateSetClass.Methods[name]
	b, ok := obj.(*object.Builtin)
	if !ok || b.Fn == nil {
		t.Fatalf("method %q is not a builtin function (got %T)", name, obj)
	}
	return b.Fn
}

// newSet calls a library's Set() constructor with the given kwargs and returns
// the resulting Set instance (or fails the test if it returned an error).
func newSet(t *testing.T, lib *object.Library, kwargs object.Kwargs) *object.Instance {
	t.Helper()
	result := lib.Functions()["Set"].Fn(context.Background(), kwargs)
	if err, ok := result.(*object.Error); ok {
		t.Fatalf("Set() returned error: %s", err.Message)
	}
	inst, ok := result.(*object.Instance)
	if !ok {
		t.Fatalf("Set() returned %T, want *object.Instance", result)
	}
	return inst
}

// addSrc calls Set.add(source) on the instance.
func addSrc(t *testing.T, inst *object.Instance, src string) {
	t.Helper()
	result := methodFn(t, "add")(context.Background(), object.NewKwargs(nil), inst, object.NewString(src))
	if err, ok := result.(*object.Error); ok {
		t.Fatalf("add(%q) returned error: %s", src, err.Message)
	}
}

// render calls Set.render(...) on the instance and returns the rendered string.
func render(t *testing.T, inst *object.Instance, args ...object.Object) string {
	t.Helper()
	fullArgs := append([]object.Object{inst}, args...)
	result := methodFn(t, "render")(context.Background(), object.NewKwargs(nil), fullArgs...)
	if err, ok := result.(*object.Error); ok {
		t.Fatalf("render() returned error: %s", err.Message)
	}
	str, ok := result.(*object.String)
	if !ok {
		t.Fatalf("render() returned %T, want *object.String", result)
	}
	return str.StringValue()
}

// renderErr calls Set.render(...) and asserts it returned an *object.Error.
func renderErr(t *testing.T, inst *object.Instance, args ...object.Object) {
	t.Helper()
	fullArgs := append([]object.Object{inst}, args...)
	result := methodFn(t, "render")(context.Background(), object.NewKwargs(nil), fullArgs...)
	if _, ok := result.(*object.Error); !ok {
		t.Fatalf("render() returned %T, want *object.Error", result)
	}
}

// --- HTML: default delimiters ----------------------------------------------

func TestTemplateHTMLDefaultRenders(t *testing.T) {
	inst := newSet(t, TemplateHTMLLibrary, object.NewKwargs(nil))
	addSrc(t, inst, "<p>{{.Name}}</p>")
	got := render(t, inst, dict("Name", "Alice"))
	if got != "<p>Alice</p>" {
		t.Fatalf("got %q, want %q", got, "<p>Alice</p>")
	}
}

// --- HTML: custom delimiters -----------------------------------------------

func TestTemplateHTMLCustomDelimiters(t *testing.T) {
	inst := newSet(t, TemplateHTMLLibrary, kw("left", "{%", "right", "%}"))
	addSrc(t, inst, "<p>{%.Name%}</p>")
	got := render(t, inst, dict("Name", "Alice"))
	if got != "<p>Alice</p>" {
		t.Fatalf("got %q, want %q", got, "<p>Alice</p>")
	}
}

func TestTemplateHTMLCustomDelimitersKeepLiteralDefaultMarkers(t *testing.T) {
	// With {% %} as delimiters, literal {{ }} must survive untouched in output.
	inst := newSet(t, TemplateHTMLLibrary, kw("left", "{%", "right", "%}"))
	addSrc(t, inst, "<p>{%.Name%} literal: {{ user.tag }}</p>")
	got := render(t, inst, dict("Name", "Alice"))
	if !strings.Contains(got, "literal: {{ user.tag }}") {
		t.Fatalf("literal {{ }} was interpreted; output: %q", got)
	}
	if !strings.Contains(got, "Alice") {
		t.Fatalf("custom action not rendered; output: %q", got)
	}
}

func TestTemplateHTMLCustomDelimitersAutoEscape(t *testing.T) {
	// Auto-escaping must still apply with custom delimiters.
	inst := newSet(t, TemplateHTMLLibrary, kw("left", "[[", "right", "]]"))
	addSrc(t, inst, "<p>[[.Content]]</p>")
	got := render(t, inst, dict("Content", "<script>alert(1)</script>"))
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Fatalf("auto-escaping not applied with custom delims; output: %q", got)
	}
	if strings.Contains(got, "<script>") {
		t.Fatalf("raw <script> survived escaping; output: %q", got)
	}
}

func TestTemplateHTMLOneSidedLeftDelimiter(t *testing.T) {
	// Override only left ("<<"); right stays as default "}}" (empty string = default).
	inst := newSet(t, TemplateHTMLLibrary, kw("left", "<<"))
	addSrc(t, inst, "<p><<.Name}}</p>")
	got := render(t, inst, dict("Name", "Bob"))
	if got != "<p>Bob</p>" {
		t.Fatalf("got %q, want %q", got, "<p>Bob</p>")
	}
}

func TestTemplateHTMLNamedTemplateCustomDelims(t *testing.T) {
	inst := newSet(t, TemplateHTMLLibrary, kw("left", "{%", "right", "%}"))
	addSrc(t, inst, `{%define "header"%}<h1>{%.Title%}</h1>{%end%}`)
	addSrc(t, inst, `{%define "page"%}{%template "header" .%}<p>{%.Body%}</p>{%end%}`)
	got := render(t, inst, object.NewString("page"), dict("Title", "Home", "Body", "Hi"))
	if !strings.Contains(got, "<h1>Home</h1>") || !strings.Contains(got, "<p>Hi</p>") {
		t.Fatalf("named template render failed; output: %q", got)
	}
}

// --- HTML: error cases -----------------------------------------------------

func TestTemplateHTMLNonStringLeftKwarg(t *testing.T) {
	result := TemplateHTMLLibrary.Functions()["Set"].Fn(context.Background(),
		kw("left", object.NewInteger(42)))
	if _, ok := result.(*object.Error); !ok {
		t.Fatalf("expected *object.Error for non-string left, got %T", result)
	}
}

func TestTemplateHTMLNonStringRightKwarg(t *testing.T) {
	result := TemplateHTMLLibrary.Functions()["Set"].Fn(context.Background(),
		kw("right", object.NewInteger(42)))
	if _, ok := result.(*object.Error); !ok {
		t.Fatalf("expected *object.Error for non-string right, got %T", result)
	}
}

func TestTemplateHTMLParseErrorWithWrongDelims(t *testing.T) {
	// An unclosed default-delimiter action must produce a parse error.
	inst := newSet(t, TemplateHTMLLibrary, object.NewKwargs(nil))
	addResult := methodFn(t, "add")(context.Background(), object.NewKwargs(nil),
		inst, object.NewString("Hello, {{.Name"))
	if _, ok := addResult.(*object.Error); !ok {
		t.Fatalf("expected parse error for unclosed action, got %T", addResult)
	}
}

func TestTemplateHTMLSetRejectsPositionalArg(t *testing.T) {
	result := TemplateHTMLLibrary.Functions()["Set"].Fn(context.Background(),
		object.NewKwargs(nil), object.NewString("unexpected"))
	if _, ok := result.(*object.Error); !ok {
		t.Fatalf("expected error for positional arg, got %T", result)
	}
}

// --- Text: default delimiters ----------------------------------------------

func TestTemplateTextDefaultRenders(t *testing.T) {
	inst := newSet(t, TemplateTextLibrary, object.NewKwargs(nil))
	addSrc(t, inst, "Hello, {{.Name}}! You have {{.Count}} messages.")
	got := render(t, inst, dict("Name", "Alice", "Count", 5))
	if got != "Hello, Alice! You have 5 messages." {
		t.Fatalf("got %q", got)
	}
}

// --- Text: custom delimiters -----------------------------------------------

func TestTemplateTextCustomDelimiters(t *testing.T) {
	inst := newSet(t, TemplateTextLibrary, kw("left", "{%", "right", "%}"))
	addSrc(t, inst, "Hello, {%.Name%}!")
	got := render(t, inst, dict("Name", "Alice"))
	if got != "Hello, Alice!" {
		t.Fatalf("got %q, want %q", got, "Hello, Alice!")
	}
}

func TestTemplateTextCustomDelimitersKeepLiteralDefaultMarkers(t *testing.T) {
	inst := newSet(t, TemplateTextLibrary, kw("left", "{%", "right", "%}"))
	addSrc(t, inst, "Hello, {%.Name%}! Upstream: {{ service.tag }}")
	got := render(t, inst, dict("Name", "Alice"))
	if !strings.Contains(got, "{{ service.tag }}") {
		t.Fatalf("literal {{ }} was interpreted; output: %q", got)
	}
	if !strings.Contains(got, "Hello, Alice!") {
		t.Fatalf("custom action not rendered; output: %q", got)
	}
}

func TestTemplateTextNoEscaping(t *testing.T) {
	// text/template must NOT escape HTML, even with custom delimiters.
	inst := newSet(t, TemplateTextLibrary, kw("left", "[[", "right", "]]"))
	addSrc(t, inst, "[[.Content]]")
	got := render(t, inst, dict("Content", "<b>raw</b>"))
	if got != "<b>raw</b>" {
		t.Fatalf("text template escaped output; got %q", got)
	}
}

func TestTemplateTextOneSidedRightDelimiter(t *testing.T) {
	// Override only right (">>"); left stays as default "{{".
	inst := newSet(t, TemplateTextLibrary, kw("right", ">>"))
	addSrc(t, inst, "{{.Name>>")
	got := render(t, inst, dict("Name", "Bob"))
	if got != "Bob" {
		t.Fatalf("got %q, want %q", got, "Bob")
	}
}

func TestTemplateTextNamedTemplateCustomDelims(t *testing.T) {
	inst := newSet(t, TemplateTextLibrary, kw("left", "{%", "right", "%}"))
	addSrc(t, inst, `{%define "greeting"%}Hello, {%.Name%}!{%end%}`)
	addSrc(t, inst, `{%define "email"%}{%template "greeting" .%} Order ready.{%end%}`)
	got := render(t, inst, object.NewString("email"), dict("Name", "Alice"))
	if !strings.Contains(got, "Hello, Alice!") {
		t.Fatalf("named text template render failed; output: %q", got)
	}
}

// --- Text: error cases -----------------------------------------------------

func TestTemplateTextNonStringLeftKwarg(t *testing.T) {
	result := TemplateTextLibrary.Functions()["Set"].Fn(context.Background(),
		kw("left", object.NewInteger(42)))
	if _, ok := result.(*object.Error); !ok {
		t.Fatalf("expected *object.Error for non-string left, got %T", result)
	}
}

func TestTemplateTextNonStringRightKwarg(t *testing.T) {
	result := TemplateTextLibrary.Functions()["Set"].Fn(context.Background(),
		kw("right", object.NewInteger(42)))
	if _, ok := result.(*object.Error); !ok {
		t.Fatalf("expected *object.Error for non-string right, got %T", result)
	}
}

func TestTemplateTextParseErrorWithWrongDelims(t *testing.T) {
	// An unclosed default-delimiter action must produce a parse error.
	inst := newSet(t, TemplateTextLibrary, object.NewKwargs(nil))
	addResult := methodFn(t, "add")(context.Background(), object.NewKwargs(nil),
		inst, object.NewString("Hello, {{.Name"))
	if _, ok := addResult.(*object.Error); !ok {
		t.Fatalf("expected parse error for unclosed action, got %T", addResult)
	}
}

func TestTemplateTextRenderUnknownNameErrors(t *testing.T) {
	inst := newSet(t, TemplateTextLibrary, object.NewKwargs(nil))
	addSrc(t, inst, `{{define "a"}}x{{end}}`)
	renderErr(t, inst, object.NewString("missing"), dict("X", 1))
}

func TestTemplateTextSetRejectsPositionalArg(t *testing.T) {
	result := TemplateTextLibrary.Functions()["Set"].Fn(context.Background(),
		object.NewKwargs(nil), object.NewString("unexpected"))
	if _, ok := result.(*object.Error); !ok {
		t.Fatalf("expected error for positional arg, got %T", result)
	}
}
