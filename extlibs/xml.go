// Package extlibs provides external libraries that need explicit registration
package extlibs

import (
	"bytes"
	"context"
	"encoding/xml"
	"sort"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// RegisterXmlLibrary registers the scriptling.xml library.
func RegisterXmlLibrary(registrar object.LibraryRegistrar) {
	registrar.RegisterLibrary(NewXmlLibrary())
}

func NewXmlLibrary() *object.Library {
	return object.NewLibrary(XmlLibraryName, map[string]*object.Builtin{
		"loads": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				content, err := args[0].AsString()
				if err != nil {
					return err
				}
				result, e := xmlLoads(content)
				if e != nil {
					return errors.NewError("xml.loads: %s", e.Error())
				}
				return result
			},
			HelpText: `loads(content) - Parse an XML string into a nested dict

Converts XML to a dict-based structure (similar to xmltodict):

- Element tags become dict keys
- Text content becomes a string value (or "#text" when the element also has
  attributes or children)
- Attributes become "@"-prefixed keys (e.g. "@id")
- Repeated child elements become list values
- Empty elements become empty strings

Parameters:
  content  XML text to parse

Returns:
  dict - Nested dict representing the XML document`,
		},
		"dumps": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				d, ok := args[0].(*object.Dict)
				if !ok {
					return errors.NewError("xml.dumps: expected a dict")
				}
				indent := kwargs.MustGetString("indent", "")
				result, e := xmlDumps(d, indent)
				if e != nil {
					return errors.NewError("xml.dumps: %s", e.Error())
				}
				return object.NewString(result)
			},
			HelpText: `dumps(data, indent="") - Format a dict into an XML string

Converts a dict-based structure back to XML. The dict should have a single root
key whose value is the root element's content. Keys prefixed with "@" become
attributes; "#text" becomes text content; list values produce repeated elements.

Parameters:
  data    Dict with a single root element key
  indent  Indentation string (default "" = compact)

Returns:
  str - XML-formatted text`,
		},
	}, nil, "XML parsing and formatting (dict-based, string-only)")
}

// ---------------------------------------------------------------------------
// loads: XML string → scriptling Dict
// ---------------------------------------------------------------------------

type xmlChildEntry struct {
	name string
	val  object.Object
}

func xmlLoads(content string) (object.Object, error) {
	dec := xml.NewDecoder(strings.NewReader(content))
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if start, ok := tok.(xml.StartElement); ok {
			val, err := parseXMLElement(dec, start)
			if err != nil {
				return nil, err
			}
			d := &object.Dict{Pairs: make(map[string]object.DictPair)}
			d.SetByString(start.Name.Local, val)
			return d, nil
		}
		// Skip comments, processing instructions, directives.
	}
}

func parseXMLElement(dec *xml.Decoder, start xml.StartElement) (object.Object, error) {
	var textBuf strings.Builder
	var children []xmlChildEntry

	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			child, err := parseXMLElement(dec, t)
			if err != nil {
				return nil, err
			}
			children = append(children, xmlChildEntry{name: t.Name.Local, val: child})
		case xml.CharData:
			textBuf.Write(t)
		case xml.EndElement:
			return buildElementResult(start, textBuf.String(), children)
		}
	}
}

func buildElementResult(start xml.StartElement, rawText string, children []xmlChildEntry) (object.Object, error) {
	textStr := strings.TrimSpace(rawText)
	hasAttrs := len(start.Attr) > 0
	hasChildren := len(children) > 0

	// Pure text element (no attrs, no children).
	if !hasAttrs && !hasChildren {
		return object.NewString(textStr), nil
	}

	d := &object.Dict{Pairs: make(map[string]object.DictPair)}

	// Attributes → @-prefixed keys.
	for _, attr := range start.Attr {
		d.SetByString("@"+attr.Name.Local, object.NewString(attr.Value))
	}

	// Group children: repeated names become lists, unique names stay single.
	counts := make(map[string]int)
	for _, c := range children {
		counts[c.name]++
	}
	lists := make(map[string]*object.List)
	for _, c := range children {
		if counts[c.name] > 1 {
			if _, exists := lists[c.name]; !exists {
				lists[c.name] = &object.List{}
			}
			lists[c.name].Elements = append(lists[c.name].Elements, c.val)
		} else {
			d.SetByString(c.name, c.val)
		}
	}
	for name, list := range lists {
		d.SetByString(name, list)
	}

	// Text content alongside attrs/children → #text key.
	if textStr != "" {
		d.SetByString("#text", object.NewString(textStr))
	}

	return d, nil
}

// ---------------------------------------------------------------------------
// dumps: scriptling Dict → XML string
// ---------------------------------------------------------------------------

type xmlAttrOut struct{ name, val string }
type xmlChildOut struct {
	name string
	val  object.Object
}

func xmlDumps(data *object.Dict, indent string) (string, error) {
	var buf bytes.Buffer
	for _, pair := range data.Pairs {
		name, _ := pair.Key.AsString()
		if err := emitXML(&buf, name, pair.Value, indent, 0); err != nil {
			return "", err
		}
		break // single root element
	}
	return buf.String(), nil
}

func emitXML(buf *bytes.Buffer, name string, val object.Object, indent string, depth int) error {
	switch v := val.(type) {
	case *object.Dict:
		var attrs []xmlAttrOut
		var children []xmlChildOut
		var text string
		hasText := false

		for _, pair := range v.Pairs {
			key, _ := pair.Key.AsString()
			if strings.HasPrefix(key, "@") {
				s, _ := pair.Value.AsString()
				attrs = append(attrs, xmlAttrOut{name: key[1:], val: s})
			} else if key == "#text" {
				text, _ = pair.Value.AsString()
				hasText = true
			} else {
				children = append(children, xmlChildOut{name: key, val: pair.Value})
			}
		}

		sort.Slice(attrs, func(i, j int) bool { return attrs[i].name < attrs[j].name })
		sort.Slice(children, func(i, j int) bool { return children[i].name < children[j].name })

		writeIndent(buf, indent, depth)
		buf.WriteByte('<')
		buf.WriteString(name)
		for _, a := range attrs {
			buf.WriteByte(' ')
			buf.WriteString(a.name)
			buf.WriteString(`="`)
			xml.EscapeText(buf, []byte(a.val))
			buf.WriteByte('"')
		}
		buf.WriteByte('>')

		if hasText {
			xml.EscapeText(buf, []byte(text))
		}

		for _, c := range children {
			if indent != "" {
				buf.WriteByte('\n')
			}
			if list, ok := c.val.(*object.List); ok {
				for _, item := range list.Elements {
					if indent != "" {
						buf.WriteByte('\n')
					}
					if err := emitXML(buf, c.name, item, indent, depth+1); err != nil {
						return err
					}
				}
			} else {
				if err := emitXML(buf, c.name, c.val, indent, depth+1); err != nil {
					return err
				}
			}
		}

		if indent != "" && len(children) > 0 {
			buf.WriteByte('\n')
			writeIndent(buf, indent, depth)
		}
		buf.WriteString("</")
		buf.WriteString(name)
		buf.WriteByte('>')

	case *object.List:
		for _, item := range v.Elements {
			if err := emitXML(buf, name, item, indent, depth); err != nil {
				return err
			}
		}

	default:
		// String, Integer, Float, Boolean — treat as text content.
		s, _ := val.AsString()
		writeIndent(buf, indent, depth)
		buf.WriteByte('<')
		buf.WriteString(name)
		buf.WriteByte('>')
		xml.EscapeText(buf, []byte(s))
		buf.WriteString("</")
		buf.WriteString(name)
		buf.WriteByte('>')
	}
	return nil
}

func writeIndent(buf *bytes.Buffer, indent string, depth int) {
	if indent != "" {
		buf.WriteString(strings.Repeat(indent, depth))
	}
}
