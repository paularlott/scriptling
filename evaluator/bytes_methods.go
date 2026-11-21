package evaluator

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// The bytes methods Python shares with str (upper, strip, find, replace,
// partition, startswith, ...) run the str implementation over a text form
// of the bytes: ASCII bytes stay themselves and bytes 0x80-0xFF become
// private-use characters, which are neither letters, digits, spaces nor
// cased, so the methods keep their ASCII-only bytes meaning. Results map
// back to bytes.

// bytesViaStrMethods are the bytes methods served this way.
var bytesViaStrMethods = map[string]bool{
	"capitalize": true, "center": true, "count": true, "endswith": true, "expandtabs": true, "find": true,
	"index": true, "isalnum": true, "isalpha": true, "isascii": true, "isdigit": true, "islower": true,
	"isspace": true, "istitle": true, "isupper": true, "ljust": true, "lower": true, "lstrip": true,
	"partition": true, "removeprefix": true, "removesuffix": true, "replace": true, "rfind": true,
	"rindex": true, "rjust": true, "rpartition": true, "rstrip": true, "startswith": true, "strip": true,
	"swapcase": true, "title": true, "upper": true, "zfill": true,
}

const bytesHighBase = 0xF700 // 0x80-0xFF map to U+F780-U+F7FF

func bytesToText(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b))
	for _, c := range b {
		if c < 0x80 {
			sb.WriteByte(c)
		} else {
			sb.WriteRune(rune(bytesHighBase + int(c)))
		}
	}
	return sb.String()
}

func textToBytes(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r < 0x80 {
			out = append(out, byte(r))
		} else {
			out = append(out, byte(r-bytesHighBase))
		}
	}
	return out
}

// bytesArgToText converts a bytes method argument: bytes become text, an int
// (find(99), count(98)) the one byte it names, a tuple of bytes a tuple of
// text (startswith((b'a', b'b'))); anything else passes through.
func bytesArgToText(a object.Object, byteInt bool) (object.Object, object.Object) {
	switch v := a.(type) {
	case *object.Bytes:
		return object.NewString(bytesToText(v.BytesValue())), nil
	case *object.Integer:
		if !byteInt {
			return a, nil
		}
		n := v.IntValue()
		if n < 0 || n > 255 {
			return nil, errors.NewValueError("byte must be in range(0, 256)")
		}
		return object.NewString(bytesToText([]byte{byte(n)})), nil
	case *object.Tuple:
		elems := make([]object.Object, len(v.Elements))
		for i, e := range v.Elements {
			b, ok := e.(*object.Bytes)
			if !ok {
				return nil, errors.NewTypeErrorTagged("a bytes-like object is required, not '%s'", getTypeName(e))
			}
			elems[i] = object.NewString(bytesToText(b.BytesValue()))
		}
		return &object.Tuple{Elements: elems}, nil
	case *object.String:
		return nil, errors.NewTypeErrorTagged("a bytes-like object is required, not 'str'")
	}
	return a, nil
}

func textResultToBytes(r object.Object) object.Object {
	switch v := r.(type) {
	case *object.String:
		return object.NewBytes(textToBytes(v.StringValue()))
	case *object.Tuple:
		elems := make([]object.Object, len(v.Elements))
		for i, e := range v.Elements {
			elems[i] = textResultToBytes(e)
		}
		return &object.Tuple{Elements: elems}
	case *object.List:
		elems := make([]object.Object, len(v.Elements))
		for i, e := range v.Elements {
			elems[i] = textResultToBytes(e)
		}
		return &object.List{Elements: elems}
	}
	return r
}

// callBytesViaStr runs a bytes method through the str implementation.
func callBytesViaStr(ctx context.Context, b *object.Bytes, method string, args []object.Object, keywords object.Kwargs, env *object.Environment) object.Object {
	text := bytesToText(b.BytesValue())
	targs := make([]object.Object, len(args))
	for i, a := range args {
		// find(99) searches for byte 99; center(14) is a width.
		byteInt := i == 0 && (method == "find" || method == "rfind" || method == "index" || method == "rindex" || method == "count")
		t, errObj := bytesArgToText(a, byteInt)
		if errObj != nil {
			return errObj
		}
		targs[i] = t
	}
	// One character per byte, so character offsets (find, index) are
	// byte offsets.
	r := callStringMethod(ctx, object.NewString(text), method, targs, keywords, env)
	if propagates(r) {
		return r
	}
	return textResultToBytes(r)
}

// bytesSplitlines is bytes.splitlines(keepends=False): only \n, \r and \r\n
// end lines in bytes.
func bytesSplitlines(b []byte, keepends bool) object.Object {
	var lines []object.Object
	start := 0
	for i := 0; i < len(b); i++ {
		if b[i] != '\n' && b[i] != '\r' {
			continue
		}
		end := i
		if b[i] == '\r' && i+1 < len(b) && b[i+1] == '\n' {
			i++
		}
		if keepends {
			end = i + 1
		}
		lines = append(lines, object.NewBytes(append([]byte(nil), b[start:end]...)))
		start = i + 1
	}
	if start < len(b) {
		lines = append(lines, object.NewBytes(append([]byte(nil), b[start:]...)))
	}
	if lines == nil {
		lines = []object.Object{}
	}
	return &object.List{Elements: lines}
}

// bytesMaketrans is bytes.maketrans(from, to): a 256-byte table mapping
// each byte of from to the byte of to at the same position.
func bytesMaketrans(args []object.Object) object.Object {
	if len(args) != 2 {
		return errors.NewTypeErrorTagged("maketrans expected 2 arguments, got %d", len(args))
	}
	from, ok1 := args[0].(*object.Bytes)
	to, ok2 := args[1].(*object.Bytes)
	if !ok1 || !ok2 {
		return errors.NewTypeErrorTagged("a bytes-like object is required")
	}
	if from.Len() != to.Len() {
		return errors.NewValueError("maketrans arguments must have same length")
	}
	table := make([]byte, 256)
	for i := range table {
		table[i] = byte(i)
	}
	for i, c := range from.BytesValue() {
		table[c] = to.BytesValue()[i]
	}
	return object.NewBytes(table)
}

// bytesTranslate is bytes.translate(table, delete=b”): table is a 256-byte
// mapping or None.
func bytesTranslate(b []byte, args []object.Object, keywords object.Kwargs) object.Object {
	if keywords.Len() > 0 {
		if err := errors.CheckKwargs("translate", keywords, "delete"); err != nil {
			return err
		}
		if d, ok := keywords.Lookup("delete"); ok {
			args = append(append([]object.Object(nil), args...), d)
		}
	}
	if len(args) < 1 || len(args) > 2 {
		return errors.NewTypeErrorTagged("translate() takes 1 or 2 arguments (%d given)", len(args))
	}
	var table []byte
	if t, ok := args[0].(*object.Bytes); ok {
		if t.Len() != 256 {
			return errors.NewValueError("translation table must be 256 characters long")
		}
		table = t.BytesValue()
	} else if args[0].Type() != object.NULL_OBJ {
		return errors.NewTypeErrorTagged("a bytes-like object is required, not '%s'", getTypeName(args[0]))
	}
	var del [256]bool
	if len(args) == 2 {
		d, ok := args[1].(*object.Bytes)
		if !ok {
			return errors.NewTypeErrorTagged("a bytes-like object is required, not '%s'", getTypeName(args[1]))
		}
		for _, c := range d.BytesValue() {
			del[c] = true
		}
	}
	out := make([]byte, 0, len(b))
	for _, c := range b {
		if del[c] {
			continue
		}
		if table != nil {
			c = table[c]
		}
		out = append(out, c)
	}
	return object.NewBytes(out)
}

// bytesHex is bytes.hex(sep=”, bytes_per_sep=1).
func bytesHex(b []byte, args []object.Object, keywords object.Kwargs) object.Object {
	if keywords.Len() > 0 {
		var errObj object.Object
		if args, errObj = foldKeywords("hex", args, keywords, []string{"sep", "bytes_per_sep"}, nil); errObj != nil {
			return errObj
		}
	}
	if len(args) > 2 {
		return errors.NewTypeErrorTagged("hex() takes at most 2 arguments (%d given)", len(args))
	}
	if errObj := checkSize("hex string", int64(len(b))*3); errObj != nil {
		return errObj
	}
	const digits = "0123456789abcdef"
	sep := ""
	per := int64(1)
	if len(args) >= 1 {
		switch s := args[0].(type) {
		case *object.String:
			sep = s.StringValue()
		case *object.Bytes:
			sep = string(s.BytesValue())
		default:
			return errors.NewTypeErrorTagged("sep must be str or bytes.")
		}
		if utf8.RuneCountInString(sep) != 1 {
			return errors.NewValueError("sep must be length 1.")
		}
	}
	if len(args) == 2 {
		n, errObj := args[1].AsInt()
		if errObj != nil {
			return errObj
		}
		per = n
	}
	var sb strings.Builder
	n := int64(len(b))
	for i, c := range b {
		if sep != "" && i > 0 && per != 0 {
			// A positive bytes_per_sep counts groups from the right, a
			// negative one from the left.
			if (per > 0 && (n-int64(i))%per == 0) || (per < 0 && int64(i)%(-per) == 0) {
				sb.WriteString(sep)
			}
		}
		sb.WriteByte(digits[c>>4])
		sb.WriteByte(digits[c&15])
	}
	return object.NewString(sb.String())
}
