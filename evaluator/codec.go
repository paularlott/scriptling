package evaluator

import (
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/paularlott/scriptling/object"
)

// decodeBytes implements bytes.decode(encoding="utf-8", errors="strict")
// with Python's codec set: utf-8, ascii, latin-1, utf-16 (BOM-aware),
// utf-16le/be. Invalid input under errors="strict" raises ValueError with
// CPython-style wording (UnicodeDecodeError is a ValueError subclass there,
// so except ValueError keeps working).
func decodeBytes(b []byte, encoding, errorsMode string) (string, object.Object) {
	switch strings.ToLower(encoding) {
	case "utf-8", "utf8", "":
		return decodeUTF8(b, errorsMode)
	case "ascii", "us-ascii":
		return decodeASCII(b, errorsMode)
	case "latin-1", "latin1", "iso-8859-1", "iso8859-1":
		var sb strings.Builder
		for _, c := range b {
			sb.WriteRune(rune(c))
		}
		return sb.String(), nil
	case "utf-16", "utf16", "utf-16-le", "utf-16le":
		be := false
		data := b
		if strings.ToLower(encoding) == "utf-16" || strings.ToLower(encoding) == "utf16" {
			if len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF {
				be = true
				data = data[2:]
			} else if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
				data = data[2:]
			}
		}
		return decodeUTF16(data, be, errorsMode, encoding)
	case "utf-16-be", "utf-16be":
		return decodeUTF16(b, true, errorsMode, encoding)
	default:
		return "", codecError("unknown encoding: %s", encoding)
	}
}

func decodeUTF8(b []byte, errorsMode string) (string, object.Object) {
	var sb strings.Builder
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size == 1 {
			switch errorsMode {
			case "ignore":
				i++
				continue
			case "replace":
				sb.WriteRune(utf8.RuneError)
				i++
				continue
			default:
				return "", codecError("'utf-8' codec can't decode byte 0x%02x in position %d: invalid start byte", b[i], i)
			}
		}
		sb.WriteRune(r)
		i += size
	}
	return sb.String(), nil
}

func decodeASCII(b []byte, errorsMode string) (string, object.Object) {
	var sb strings.Builder
	for i, c := range b {
		if c < 128 {
			sb.WriteByte(c)
			continue
		}
		switch errorsMode {
		case "ignore":
		case "replace":
			sb.WriteByte('?')
		default:
			return "", codecError("'ascii' codec can't decode byte 0x%02x in position %d: ordinal not in range(128)", c, i)
		}
	}
	return sb.String(), nil
}

func decodeUTF16(b []byte, be bool, errorsMode, encoding string) (string, object.Object) {
	if len(b)%2 != 0 {
		if errorsMode != "ignore" && errorsMode != "replace" {
			return "", codecError("'%s' codec can't decode byte in position %d: truncated data", encoding, len(b)-1)
		}
		b = b[:len(b)-1]
	}
	units := make([]uint16, len(b)/2)
	for i := range units {
		if be {
			units[i] = uint16(b[i*2])<<8 | uint16(b[i*2+1])
		} else {
			units[i] = uint16(b[i*2]) | uint16(b[i*2+1])<<8
		}
	}
	return string(utf16.Decode(units)), nil
}

// encodeStr implements str.encode(encoding="utf-8", errors="strict") with
// the same codec set as decodeBytes; 'utf-16' encodes with a BOM (LE,
// CPython's native order), utf-16le/be without.
func encodeStr(s string, encoding, errorsMode string) ([]byte, object.Object) {
	switch strings.ToLower(encoding) {
	case "utf-8", "utf8", "":
		return []byte(s), nil
	case "ascii", "us-ascii":
		out := make([]byte, 0, len(s))
		for i, r := range s {
			if r < 128 {
				out = append(out, byte(r))
				continue
			}
			switch errorsMode {
			case "ignore":
			case "replace":
				out = append(out, '?')
			default:
				return nil, codecError("'ascii' codec can't encode character %s in position %d: ordinal not in range(128)", escapeRune(r), i)
			}
		}
		return out, nil
	case "latin-1", "latin1", "iso-8859-1", "iso8859-1":
		out := make([]byte, 0, len(s))
		for i, r := range s {
			if r <= 0xFF {
				out = append(out, byte(r))
				continue
			}
			switch errorsMode {
			case "ignore":
			case "replace":
				out = append(out, '?')
			default:
				return nil, codecError("'latin-1' codec can't encode character %s in position %d: ordinal not in range(256)", escapeRune(r), i)
			}
		}
		return out, nil
	case "utf-16", "utf16":
		units := utf16.Encode([]rune(s))
		out := make([]byte, 2+2*len(units))
		out[0], out[1] = 0xFF, 0xFE // BOM, little-endian like CPython native
		for i, u := range units {
			out[2+2*i] = byte(u)
			out[3+2*i] = byte(u >> 8)
		}
		return out, nil
	case "utf-16-le", "utf-16le", "utf-16-be", "utf-16be":
		be := strings.Contains(strings.ToLower(encoding), "be")
		units := utf16.Encode([]rune(s))
		out := make([]byte, 2*len(units))
		for i, u := range units {
			if be {
				out[2*i] = byte(u >> 8)
				out[2*i+1] = byte(u)
			} else {
				out[2*i] = byte(u)
				out[2*i+1] = byte(u >> 8)
			}
		}
		return out, nil
	default:
		return nil, codecError("unknown encoding: %s", encoding)
	}
}

// escapeRune renders a character the way CPython's codec errors do: \xe9,
// \u20ac or \U0001f600.
func escapeRune(r rune) string {
	switch {
	case r < 0x100:
		return fmt.Sprintf("\\x%02x", r)
	case r < 0x10000:
		return fmt.Sprintf("\\u%04x", r)
	default:
		return fmt.Sprintf("\\U%08x", r)
	}
}

func codecError(format string, a ...any) object.Object {
	err := codecErrValue(format, a...)
	err.ExceptionType = object.ExceptionTypeValueError
	return err
}

func codecErrValue(format string, a ...any) *object.Exception {
	return &object.Exception{Message: fmt.Sprintf(format, a...), Raised: true}
}
