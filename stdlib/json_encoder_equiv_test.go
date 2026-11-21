package stdlib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// referenceJSONString is the string escaper json.dumps used before the
// streaming encoder: the new one must match it byte for byte.
func referenceJSONString(s string, ensureAscii bool) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\f':
			b.WriteString(`\f`)
		case r < 0x20:
			fmt.Fprintf(&b, `\u%04x`, r)
		case ensureAscii && r > 0x7F:
			if r > 0xFFFF {
				hi := 0xD800 + ((r - 0x10000) >> 10)
				lo := 0xDC00 + ((r - 0x10000) & 0x3FF)
				fmt.Fprintf(&b, `\u%04x\u%04x`, hi, lo)
			} else {
				fmt.Fprintf(&b, `\u%04x`, r)
			}
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func randomJSONTestString(rng *rand.Rand) string {
	pieces := []string{"a", "Z", " ", "\"", "\\", "\n", "\r", "\t", "\b", "\f", "\x00", "\x1f", "\x7f",
		"é", "ß", "中", "😀", " ", "\xff", "\xe4\xb8", "<", ">", "&", ",", ":", "{", "]"}
	var b strings.Builder
	for n := rng.Intn(12); n > 0; n-- {
		b.WriteString(pieces[rng.Intn(len(pieces))])
	}
	return b.String()
}

func TestJSONStringEscapingMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 20000; i++ {
		s := randomJSONTestString(rng)
		for _, ascii := range []bool{true, false} {
			var got bytes.Buffer
			writeJSONString(&got, s, ascii)
			if want := referenceJSONString(s, ascii); got.String() != want {
				t.Fatalf("writeJSONString(%q, %v) = %s, want %s", s, ascii, got.String(), want)
			}
		}
	}
}

func randomJSONTestValue(rng *rand.Rand, depth int) object.Object {
	k := rng.Intn(9)
	if depth > 4 {
		k = rng.Intn(5)
	}
	switch k {
	case 0:
		return object.NewInteger(rng.Int63n(2000) - 1000)
	case 1:
		return object.NewFloat(rng.NormFloat64() * 1e3)
	case 2:
		return object.NewString(randomJSONTestString(rng))
	case 3:
		return object.NewBoolean(rng.Intn(2) == 0)
	case 4:
		return &object.Null{}
	case 5, 6:
		n := rng.Intn(4)
		elems := make([]object.Object, n)
		for i := range elems {
			elems[i] = randomJSONTestValue(rng, depth+1)
		}
		if k == 5 {
			return &object.List{Elements: elems}
		}
		return &object.Tuple{Elements: elems}
	default:
		d := object.NewStringDict(nil)
		for n := rng.Intn(4); n > 0; n-- {
			d.SetByString(randomJSONTestString(rng), randomJSONTestValue(rng, depth+1))
		}
		return d
	}
}

// TestJSONIndentMatchesEncodingJSON checks the bounded indenter lays out
// valid JSON exactly as json.Indent, which json.dumps used before.
func TestJSONIndentMatchesEncodingJSON(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	seps := [][2]string{{",", ": "}, {", ", ": "}, {",", ":"}, {" , ", " : "}}
	for i := 0; i < 5000; i++ {
		v := randomJSONTestValue(rng, 0)
		sep := seps[rng.Intn(len(seps))]
		enc := jsonEncoder{opts: jsonOpts{itemSep: sep[0], keySep: sep[1], ensureAscii: rng.Intn(2) == 0}}
		if errObj := enc.value(v, map[object.Object]struct{}{}, 0); errObj != nil {
			t.Fatalf("encode: %s", errObj.Inspect())
		}
		compact := enc.buf.Bytes()
		if !json.Valid(compact) {
			t.Fatalf("invalid JSON: %s", compact)
		}
		for _, indent := range []string{"", "  ", "\t", "--"} {
			var want bytes.Buffer
			if err := json.Indent(&want, compact, "", indent); err != nil {
				t.Fatal(err)
			}
			got, errObj := indentJSON(compact, indent, ",", ": ")
			if errObj != nil {
				t.Fatalf("indentJSON: %s", errObj.Inspect())
			}
			if !bytes.Equal(got, want.Bytes()) {
				t.Fatalf("indentJSON(%s, %q)\n got: %s\nwant: %s", compact, indent, got, want.Bytes())
			}
		}
	}
}
