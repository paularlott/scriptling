// Package extlibs provides external libraries that need explicit registration
package extlibs

import (
	"context"
	"regexp"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// RegisterShlexLibrary registers the shlex library with a Scriptling instance.
func RegisterShlexLibrary(registrar object.LibraryRegistrar) {
	registrar.RegisterLibrary(NewShlexLibrary())
}

// NewShlexLibrary creates a new shlex library.
func NewShlexLibrary() *object.Library {
	return object.NewLibrary(ShlexLibraryName, map[string]*object.Builtin{
		"quote": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				s, err := args[0].AsString()
				if err != nil {
					return err
				}
				return object.NewString(shlexQuote(s))
			},
			HelpText: `quote(s) - Escape a string for use as a single shell argument

Returns a shell-escaped version of the string. The returned value is safe to
embed in a shell command line as a single argument. If the string contains only
safe characters it is returned unchanged; otherwise it is wrapped in single
quotes with any embedded single quotes escaped.`,
		},
		"split": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				s, err := args[0].AsString()
				if err != nil {
					return err
				}
				tokens, e := shlexSplit(s)
				if e != nil {
					return errors.NewError("shlex.split: %s", e.Error())
				}
				elements := make([]object.Object, len(tokens))
				for i, t := range tokens {
					elements[i] = object.NewString(t)
				}
				return &object.List{Elements: elements}
			},
			HelpText: `split(s) - Split a string into shell-style tokens

Parses the string using shell-style rules for quoting and escaping, returning a
list of tokens. Single quotes preserve everything literally; double quotes
preserve everything except a backslash before a special character; outside
quotes a backslash escapes the next character.`,
		},
		"join": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				list, err := args[0].AsList()
				if err != nil {
					return err
				}
				parts := make([]string, len(list))
				for i, elem := range list {
					s, e := elem.AsString()
					if e != nil {
						return e
					}
					parts[i] = s
				}
				return object.NewString(shlexJoin(parts))
			},
			HelpText: `join(split_command) - Join command args into a shell-quoted string

Each argument is individually quoted with quote() and joined with spaces,
producing a single shell-safe command line string.`,
		},
	}, nil, "Shell-style lexical analysis and quoting")
}

var shlexUnsafe = regexp.MustCompile(`[^a-zA-Z0-9_@%+=:,./\-]`)

// shlexQuote wraps a string in single quotes if it contains any character that
// is not shell-safe.
func shlexQuote(s string) string {
	if s == "" {
		return "''"
	}
	if !shlexUnsafe.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// shlexSplit parses a shell-style command string into tokens.
func shlexSplit(s string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	inSingle, inDouble, escaped := false, false, false
	hasToken := false

	for i := 0; i < len(s); i++ {
		c := s[i]

		if escaped {
			current.WriteByte(c)
			escaped = false
			hasToken = true
			continue
		}

		if c == '\\' && !inSingle {
			if inDouble {
				// Inside double quotes, backslash only escapes special chars.
				if i+1 < len(s) {
					next := s[i+1]
					if next == '"' || next == '\\' || next == '$' || next == '`' || next == '\n' {
						escaped = true
						hasToken = true
						continue
					}
				}
				current.WriteByte(c)
				hasToken = true
				continue
			}
			escaped = true
			hasToken = true
			continue
		}

		switch {
		case c == '\'' && !inDouble:
			inSingle = !inSingle
			hasToken = true
		case c == '"' && !inSingle:
			inDouble = !inDouble
			hasToken = true
		case (c == ' ' || c == '\t' || c == '\n') && !inSingle && !inDouble:
			if hasToken {
				tokens = append(tokens, current.String())
				current.Reset()
				hasToken = false
			}
		default:
			current.WriteByte(c)
			hasToken = true
		}
	}

	if escaped {
		return nil, errShlex("trailing backslash")
	}
	if inSingle {
		return nil, errShlex("unterminated single quote")
	}
	if inDouble {
		return nil, errShlex("unterminated double quote")
	}
	if hasToken {
		tokens = append(tokens, current.String())
	}
	return tokens, nil
}

// shlexJoin quotes each argument and joins with spaces.
func shlexJoin(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = shlexQuote(a)
	}
	return strings.Join(quoted, " ")
}

type errShlex string

func (e errShlex) Error() string { return string(e) }
