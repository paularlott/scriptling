package stdlib

import (
	"container/list"
	"context"
	"fmt"
	"regexp"
	"sync"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// Flag constants matching Python's re module
const (
	RE_IGNORECASE = 2  // re.I or re.IGNORECASE
	RE_MULTILINE  = 8  // re.M or re.MULTILINE
	RE_DOTALL     = 16 // re.S or re.DOTALL
)

type regexEntry struct {
	pattern string
	regex   *regexp.Regexp
	element *list.Element
}

type regexCache struct {
	mu      sync.RWMutex
	entries map[string]*regexEntry
	lru     *list.List
	maxSize int
}

var globalRegexCache = &regexCache{
	entries: make(map[string]*regexEntry),
	lru:     list.New(),
	maxSize: 100, // Max 100 cached regex patterns
}

// applyFlags converts Python-style flags to Go regex inline flags
func applyFlags(pattern string, flags int64) string {
	prefix := ""
	if flags&RE_IGNORECASE != 0 {
		prefix += "i"
	}
	if flags&RE_MULTILINE != 0 {
		prefix += "m"
	}
	if flags&RE_DOTALL != 0 {
		prefix += "s"
	}
	if prefix != "" {
		return fmt.Sprintf("(?%s)%s", prefix, pattern)
	}
	return pattern
}

// getCompiledRegex retrieves a compiled regex from cache or compiles and caches it
func getCompiledRegex(pattern string) (*regexp.Regexp, error) {
	globalRegexCache.mu.RLock()
	if entry, ok := globalRegexCache.entries[pattern]; ok {
		// Move to front (most recently used)
		globalRegexCache.mu.RUnlock()
		globalRegexCache.mu.Lock()
		globalRegexCache.lru.MoveToFront(entry.element)
		globalRegexCache.mu.Unlock()
		return entry.regex, nil
	}
	globalRegexCache.mu.RUnlock()

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	globalRegexCache.mu.Lock()
	defer globalRegexCache.mu.Unlock()

	// Check again in case another goroutine added it
	if entry, ok := globalRegexCache.entries[pattern]; ok {
		globalRegexCache.lru.MoveToFront(entry.element)
		return entry.regex, nil
	}

	// Evict old entries if cache is full
	for len(globalRegexCache.entries) >= globalRegexCache.maxSize {
		globalRegexCache.evictOldest()
	}

	// Add new entry at front
	entry := &regexEntry{
		pattern: pattern,
		regex:   re,
	}
	elem := globalRegexCache.lru.PushFront(entry)
	entry.element = elem
	globalRegexCache.entries[pattern] = entry

	return re, nil
}

func (c *regexCache) evictOldest() {
	elem := c.lru.Back()
	if elem == nil {
		return
	}

	entry := elem.Value.(*regexEntry)
	c.lru.Remove(elem)
	delete(c.entries, entry.pattern)
}

// Helper to extract optional flags argument
func getFlags(args []object.Object, flagsIndex int) (int64, error) {
	if len(args) <= flagsIndex {
		return 0, nil
	}
	if args[flagsIndex].Type() != object.INTEGER_OBJ {
		return 0, fmt.Errorf("flags must be an integer")
	}
	val, _ := args[flagsIndex].AsInt()
	return val, nil
}

var ReLibrary = object.NewLibrary(map[string]*object.Builtin{
	"match": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) < 2 || len(args) > 3 {
				return errors.NewError("match() takes 2 or 3 arguments (%d given)", len(args))
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern, _ := args[0].AsString()
			text, _ := args[1].AsString()

			flags, err := getFlags(args, 2)
			if err != nil {
				return errors.NewError("%s", err.Error())
			}
			pattern = applyFlags(pattern, flags)

			re, err := getCompiledRegex(pattern)
			if err != nil {
				return errors.NewError("regex compile error: %s", err.Error())
			}

			// Check if pattern matches at the beginning of text
			loc := re.FindStringIndex(text)
			if loc == nil || loc[0] != 0 {
				return &object.Boolean{Value: false}
			}
			return &object.Boolean{Value: true}
		},
		HelpText: `match(pattern, string, flags=0) - Match pattern at start of string

Returns true if the regex pattern matches at the beginning of the string.

Flags:
  re.IGNORECASE or re.I - Case-insensitive matching
  re.MULTILINE or re.M  - ^ and $ match at line boundaries
  re.DOTALL or re.S     - . matches newlines`,
	},
	"search": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) < 2 || len(args) > 3 {
				return errors.NewError("search() takes 2 or 3 arguments (%d given)", len(args))
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern, _ := args[0].AsString()
			text, _ := args[1].AsString()

			flags, err := getFlags(args, 2)
			if err != nil {
				return errors.NewError("%s", err.Error())
			}
			pattern = applyFlags(pattern, flags)

			re, err := getCompiledRegex(pattern)
			if err != nil {
				return errors.NewError("regex compile error: %s", err.Error())
			}

			result := re.FindString(text)
			if result == "" {
				return &object.Null{}
			}
			return &object.String{Value: result}
		},
		HelpText: `search(pattern, string, flags=0) - Search for pattern

Returns the first substring that matches the regex pattern anywhere in the string, or null if no match.

Flags:
  re.IGNORECASE or re.I - Case-insensitive matching
  re.MULTILINE or re.M  - ^ and $ match at line boundaries
  re.DOTALL or re.S     - . matches newlines`,
	},
	"findall": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) < 2 || len(args) > 3 {
				return errors.NewError("findall() takes 2 or 3 arguments (%d given)", len(args))
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern, _ := args[0].AsString()
			text, _ := args[1].AsString()

			flags, err := getFlags(args, 2)
			if err != nil {
				return errors.NewError("%s", err.Error())
			}
			pattern = applyFlags(pattern, flags)

			re, err := getCompiledRegex(pattern)
			if err != nil {
				return errors.NewError("regex compile error: %s", err.Error())
			}

			matches := re.FindAllString(text, -1)
			elements := make([]object.Object, len(matches))
			for i, match := range matches {
				elements[i] = &object.String{Value: match}
			}
			return &object.List{Elements: elements}
		},
		HelpText: `findall(pattern, string, flags=0) - Find all matches

Returns a list of all substrings that match the regex pattern.

Flags:
  re.IGNORECASE or re.I - Case-insensitive matching
  re.MULTILINE or re.M  - ^ and $ match at line boundaries
  re.DOTALL or re.S     - . matches newlines`,
	},
	"sub": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) < 3 || len(args) > 5 {
				return errors.NewError("sub() takes 3 to 5 arguments (%d given)", len(args))
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ || args[2].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern, _ := args[0].AsString()
			replacement, _ := args[1].AsString()
			text, _ := args[2].AsString()

			// count parameter (optional, position 3)
			count := -1 // -1 means replace all
			if len(args) > 3 {
				if args[3].Type() != object.INTEGER_OBJ {
					return errors.NewError("count must be an integer")
				}
				val, _ := args[3].AsInt()
				count = int(val)
			}

			// flags parameter (optional, position 4)
			flags, err := getFlags(args, 4)
			if err != nil {
				return errors.NewError("%s", err.Error())
			}
			pattern = applyFlags(pattern, flags)

			re, err := getCompiledRegex(pattern)
			if err != nil {
				return errors.NewError("regex compile error: %s", err.Error())
			}

			var result string
			if count == 0 {
				result = text
			} else if count < 0 {
				result = re.ReplaceAllString(text, replacement)
			} else {
				// Replace only 'count' occurrences
				replaced := 0
				result = re.ReplaceAllStringFunc(text, func(match string) string {
					if replaced < count {
						replaced++
						return re.ReplaceAllString(match, replacement)
					}
					return match
				})
			}
			return &object.String{Value: result}
		},
		HelpText: `sub(pattern, repl, string, count=0, flags=0) - Replace matches

Replaces occurrences of the regex pattern in the string with the replacement.
If count is 0 (default), all occurrences are replaced.
If count > 0, only the first count occurrences are replaced.

Flags:
  re.IGNORECASE or re.I - Case-insensitive matching
  re.MULTILINE or re.M  - ^ and $ match at line boundaries
  re.DOTALL or re.S     - . matches newlines`,
	},
	"split": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) < 2 || len(args) > 4 {
				return errors.NewError("split() takes 2 to 4 arguments (%d given)", len(args))
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern, _ := args[0].AsString()
			text, _ := args[1].AsString()

			// maxsplit parameter (optional, position 2)
			maxsplit := -1 // -1 means no limit
			if len(args) > 2 {
				if args[2].Type() != object.INTEGER_OBJ {
					return errors.NewError("maxsplit must be an integer")
				}
				val, _ := args[2].AsInt()
				maxsplit = int(val)
				if maxsplit == 0 {
					maxsplit = -1 // 0 means no limit in Python
				}
			}

			// flags parameter (optional, position 3)
			flags, err := getFlags(args, 3)
			if err != nil {
				return errors.NewError("%s", err.Error())
			}
			pattern = applyFlags(pattern, flags)

			re, err := getCompiledRegex(pattern)
			if err != nil {
				return errors.NewError("regex compile error: %s", err.Error())
			}

			parts := re.Split(text, maxsplit)
			elements := make([]object.Object, len(parts))
			for i, part := range parts {
				elements[i] = &object.String{Value: part}
			}
			return &object.List{Elements: elements}
		},
		HelpText: `split(pattern, string, maxsplit=0, flags=0) - Split string by pattern

Splits the string by occurrences of the regex pattern and returns a list of substrings.
If maxsplit is 0 (default), all occurrences are split.
If maxsplit > 0, at most maxsplit splits are done.

Flags:
  re.IGNORECASE or re.I - Case-insensitive matching
  re.MULTILINE or re.M  - ^ and $ match at line boundaries
  re.DOTALL or re.S     - . matches newlines`,
	},
	"compile": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return errors.NewError("compile() takes 1 or 2 arguments (%d given)", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			pattern, _ := args[0].AsString()

			flags, err := getFlags(args, 1)
			if err != nil {
				return errors.NewError("%s", err.Error())
			}
			pattern = applyFlags(pattern, flags)

			// Try to compile to validate the pattern
			_, err = getCompiledRegex(pattern)
			if err != nil {
				return errors.NewError("regex compile error: %s", err.Error())
			}

			// Return the pattern string as a compiled "object"
			return &object.String{Value: pattern}
		},
		HelpText: `compile(pattern, flags=0) - Compile regex pattern

Validates and caches a regex pattern for later use. Returns the pattern if valid.

Flags:
  re.IGNORECASE or re.I - Case-insensitive matching
  re.MULTILINE or re.M  - ^ and $ match at line boundaries
  re.DOTALL or re.S     - . matches newlines`,
	},
	"escape": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			text, _ := args[0].AsString()

			escaped := regexp.QuoteMeta(text)
			return &object.String{Value: escaped}
		},
		HelpText: `escape(pattern) - Escape special regex characters

Returns a string with all special regex characters escaped.`,
	},
	"fullmatch": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) < 2 || len(args) > 3 {
				return errors.NewError("fullmatch() takes 2 or 3 arguments (%d given)", len(args))
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern, _ := args[0].AsString()
			text, _ := args[1].AsString()

			flags, err := getFlags(args, 2)
			if err != nil {
				return errors.NewError("%s", err.Error())
			}
			pattern = applyFlags(pattern, flags)

			re, err := getCompiledRegex(pattern)
			if err != nil {
				return errors.NewError("regex compile error: %s", err.Error())
			}

			// Check if the entire string matches
			loc := re.FindStringIndex(text)
			if loc == nil || loc[0] != 0 || loc[1] != len(text) {
				return &object.Boolean{Value: false}
			}
			return &object.Boolean{Value: true}
		},
		HelpText: `fullmatch(pattern, string, flags=0) - Match entire string

Returns true if the regex pattern matches the entire string.

Flags:
  re.IGNORECASE or re.I - Case-insensitive matching
  re.MULTILINE or re.M  - ^ and $ match at line boundaries
  re.DOTALL or re.S     - . matches newlines`,
	},
}, map[string]object.Object{
	// Flag constants - matching Python's re module values
	"IGNORECASE": &object.Integer{Value: RE_IGNORECASE},
	"I":          &object.Integer{Value: RE_IGNORECASE},
	"MULTILINE":  &object.Integer{Value: RE_MULTILINE},
	"M":          &object.Integer{Value: RE_MULTILINE},
	"DOTALL":     &object.Integer{Value: RE_DOTALL},
	"S":          &object.Integer{Value: RE_DOTALL},
}, "Regular expression library")
