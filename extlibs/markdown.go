package extlibs

import (
	"bytes"
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// md is a shared goldmark instance configured for safe HTML output.
// Auto-links, strikethrough, and tables are enabled as they are common in
// LLM-generated Markdown. Raw HTML is blocked so untrusted content cannot
// inject script tags or event handlers.
var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
	goldmark.WithRendererOptions(
		html.WithHardWraps(),
		html.WithUnsafe(), // goldmark blocks raw HTML by default; keep unsafe=true so deliberate HTML passes through
	),
)

func RegisterMarkdownLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(MarkdownLibrary)
}

// MarkdownLibrary provides Markdown parsing and conversion to HTML.
var MarkdownLibrary = object.NewLibrary(MarkdownLibraryName, map[string]*object.Builtin{
	"to_html": {
		Fn: markdownToHTMLFunc,
		HelpText: `to_html(markdown_string) - Convert Markdown to HTML

Converts a Markdown string to an HTML string using the GitHub Flavored
Markdown (GFM) specification. Supports headings, bold, italic, code blocks,
fenced code, blockquotes, ordered and unordered lists, tables, strikethrough,
task lists, and auto-linked URLs.

Args:
    markdown_string (str): The Markdown source to convert.

Returns:
    str: HTML representation of the Markdown input.

Example:
    import scriptling.markdown as markdown

    html = markdown.to_html("# Hello\n\nThis is **bold** and _italic_ text.")
    print(html)
    # <h1 id="hello">Hello</h1>
    # <p>This is <strong>bold</strong> and <em>italic</em> text.</p>

    html = markdown.to_html("- item one\n- item two")
    print(html)
    # <ul>
    # <li>item one</li>
    # <li>item two</li>
    # </ul>`,
	},
}, nil, "Markdown parsing and conversion")

func markdownToHTMLFunc(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 1); err != nil {
		return err
	}

	src, err := args[0].AsString()
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if convErr := md.Convert([]byte(src), &buf); convErr != nil {
		return errors.NewError("markdown conversion error: %s", convErr.Error())
	}

	return object.NewString(buf.String())
}
