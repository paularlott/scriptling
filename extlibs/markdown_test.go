package extlibs

import (
	"context"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestMarkdownToHTML(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		input    string
		contains []string // substrings that MUST appear in the output
	}{
		{
			name:     "ATX heading with auto id",
			input:    "# Title",
			contains: []string{`<h1 id="title">Title</h1>`},
		},
		{
			name:     "bold italic strikethrough",
			input:    "**b** _i_ ~~s~~",
			contains: []string{`<strong>b</strong>`, `<em>i</em>`, `<del>s</del>`},
		},
		{
			name:     "unordered list",
			input:    "- one\n- two",
			contains: []string{`<ul>`, `<li>one</li>`, `<li>two</li>`, `</ul>`},
		},
		{
			name:     "ordered list",
			input:    "1. first\n2. second",
			contains: []string{`<ol>`, `<li>first</li>`, `</ol>`},
		},
		{
			name:     "blockquote",
			input:    "> quoted",
			contains: []string{`<blockquote>`, `quoted`, `</blockquote>`},
		},
		{
			name:     "fenced code block with language",
			input:    "```python\nprint('hi')\n```",
			contains: []string{`<pre><code class="language-python">`, "print('hi')"},
		},
		{
			name:     "inline code",
			input:    "use `fmt`",
			contains: []string{`<code>fmt</code>`},
		},
		{
			name:     "GFM table",
			input:    "| a | b |\n|---|---|\n| 1 | 2 |",
			contains: []string{`<table>`, `<th>a</th>`, `<th>b</th>`, `<td>1</td>`, `<td>2</td>`, `</table>`},
		},
		{
			name:     "task list",
			input:    "- [x] done\n- [ ] todo",
			contains: []string{`<input checked="" disabled="" type="checkbox">`, `<input disabled="" type="checkbox">`},
		},
		{
			name:     "autolink bare url",
			input:    "see https://example.com here",
			contains: []string{`<a href="https://example.com">https://example.com</a>`},
		},
		{
			name:     "hard wrap newline",
			input:    "line one\nline two",
			contains: []string{`line one<br>`, "line two"},
		},
		{
			name:     "link",
			input:    "[ex](https://example.com)",
			contains: []string{`<a href="https://example.com">ex</a>`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := markdownToHTMLFunc(ctx, object.NewKwargs(nil), object.NewString(tt.input))
			if err, ok := result.(*object.Error); ok {
				t.Fatalf("unexpected error: %s", err.Message)
			}
			str, ok := result.(*object.String)
			if !ok {
				t.Fatalf("expected *object.String, got %T (%v)", result, result)
			}
			got := str.StringValue()
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q\noutput: %s", want, got)
				}
			}
		})
	}
}

// TestMarkdownRawHTMLIsSanitized is the security regression test. LLM output is
// untrusted, so raw HTML must never survive into the rendered HTML — otherwise
// an LLM (or attacker) could inject <script> tags, event handlers, etc. Goldmark
// drops raw HTML blocks and inline raw HTML by default; this test pins that
// behaviour so it cannot silently regress (e.g. by re-enabling html.WithUnsafe).
func TestMarkdownRawHTMLIsSanitized(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		input          string
		mustNotContain []string // substrings that MUST NOT appear in the output
	}{
		{
			name:           "script tag block",
			input:          "<script>alert(1)</script>",
			mustNotContain: []string{"<script", "</script>", "alert(1)"},
		},
		{
			name:           "inline img onerror XSS",
			input:          "hi <img src=x onerror=alert(1)> there",
			mustNotContain: []string{"<img", "onerror", "alert(1)"},
		},
		{
			name:           "div block",
			input:          "<div>raw block</div>",
			mustNotContain: []string{"<div", "</div>", "raw block"},
		},
		{
			name:           "iframe embed",
			input:          "<iframe src='https://evil.example'></iframe>",
			mustNotContain: []string{"<iframe", "evil.example"},
		},
		{
			name:           "event handler attribute",
			input:          "<a href='#' onclick='alert(1)'>x</a>",
			mustNotContain: []string{"onclick", "<a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := markdownToHTMLFunc(ctx, object.NewKwargs(nil), object.NewString(tt.input))
			if err, ok := result.(*object.Error); ok {
				t.Fatalf("unexpected error: %s", err.Message)
			}
			str, ok := result.(*object.String)
			if !ok {
				t.Fatalf("expected *object.String, got %T (%v)", result, result)
			}
			got := str.StringValue()
			for _, banned := range tt.mustNotContain {
				if strings.Contains(got, banned) {
					t.Errorf("output must not contain %q — raw HTML survived sanitisation\noutput: %s", banned, got)
				}
			}
		})
	}
}

// TestMarkdownCodeContentIsEscaped ensures angle brackets inside code spans and
// fenced code blocks are HTML-escaped. This is what keeps code samples (which
// commonly contain HTML-like syntax) safe to display without enabling raw HTML.
func TestMarkdownCodeContentIsEscaped(t *testing.T) {
	ctx := context.Background()

	t.Run("code span", func(t *testing.T) {
		result := markdownToHTMLFunc(ctx, object.NewKwargs(nil), object.NewString("use `<b>` here"))
		str, ok := result.(*object.String)
		if !ok {
			t.Fatalf("expected *object.String, got %T", result)
		}
		got := str.StringValue()
		if !strings.Contains(got, "&lt;b&gt;") {
			t.Errorf("expected escaped &lt;b&gt; in output\noutput: %s", got)
		}
		if strings.Contains(got, "<b>") {
			t.Errorf("raw <b> survived inside code span\noutput: %s", got)
		}
	})

	t.Run("fenced code", func(t *testing.T) {
		result := markdownToHTMLFunc(ctx, object.NewKwargs(nil), object.NewString("```html\n<b>raw</b>\n```"))
		str, ok := result.(*object.String)
		if !ok {
			t.Fatalf("expected *object.String, got %T", result)
		}
		got := str.StringValue()
		if !strings.Contains(got, "&lt;b&gt;raw&lt;/b&gt;") {
			t.Errorf("expected escaped entities in fenced code\noutput: %s", got)
		}
	})
}

func TestMarkdownErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("no args", func(t *testing.T) {
		result := markdownToHTMLFunc(ctx, object.NewKwargs(nil))
		if _, ok := result.(*object.Error); !ok {
			t.Errorf("expected error for missing arg, got %T (%v)", result, result)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		result := markdownToHTMLFunc(ctx, object.NewKwargs(nil), object.NewInteger(42))
		if _, ok := result.(*object.Error); !ok {
			t.Errorf("expected error for non-string arg, got %T (%v)", result, result)
		}
	})
}
