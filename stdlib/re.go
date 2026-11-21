package stdlib

import (
	"container/list"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/libutil"
	"github.com/paularlott/scriptling/object"
)

// Flag constants matching Python's re module
const (
	RE_IGNORECASE = 2  // re.I or re.IGNORECASE
	RE_MULTILINE  = 8  // re.M or re.MULTILINE
	RE_DOTALL     = 16 // re.S or re.DOTALL
)

// Regex class definition
var RegexClass = &object.Class{
	Name: "Regex",
	Methods: map[string]object.Object{
		"match": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("match", kwargs); err != nil {
					return err
				}
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				if args[1].Type() != object.STRING_OBJ {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}
				regex := args[0].(*object.Instance)
				pattern := regex.Field("pattern").(*object.String).StringValue()
				text, _ := args[1].AsString()

				re, err := GetCompiledRegex(pattern)
				if err != nil {
					return reCompileError(err)
				}

				// Check if pattern matches at the beginning of text
				match := re.FindStringSubmatchIndex(text)
				if match == nil || match[0] != 0 {
					return &object.Null{}
				}

				return createMatchInstance(text, match, re.SubexpNames(), &runeOffsets{text: text})
			},
			HelpText: `match(string) - Match pattern at start of string

Returns a Match object if the pattern matches at the beginning, or None if no match.`,
		},
		"search": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("search", kwargs); err != nil {
					return err
				}
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				if args[1].Type() != object.STRING_OBJ {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}
				regex := args[0].(*object.Instance)
				pattern := regex.Field("pattern").(*object.String).StringValue()
				text, _ := args[1].AsString()

				re, err := GetCompiledRegex(pattern)
				if err != nil {
					return reCompileError(err)
				}

				match := re.FindStringSubmatchIndex(text)
				if match == nil {
					return &object.Null{}
				}

				return createMatchInstance(text, match, re.SubexpNames(), &runeOffsets{text: text})
			},
			HelpText: `search(string) - Search for pattern anywhere in string

Returns a Match object for the first match, or None if no match found.`,
		},
		"findall": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("findall", kwargs); err != nil {
					return err
				}
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				if args[1].Type() != object.STRING_OBJ {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}
				regex := args[0].(*object.Instance)
				pattern := regex.Field("pattern").(*object.String).StringValue()
				text, _ := args[1].AsString()

				re, err := GetCompiledRegex(pattern)
				if err != nil {
					return reCompileError(err)
				}

				numGroups := re.NumSubexp()

				if numGroups == 0 {
					matches := re.FindAllString(text, -1)
					elements := make([]object.Object, len(matches))
					object.TrackSlice(elements)
					for i, match := range matches {
						elements[i] = object.NewSubstring(match, text)
					}
					return &object.List{Elements: elements}
				}

				matches := re.FindAllStringSubmatchIndex(text, -1)
				elements := make([]object.Object, len(matches))
				object.TrackSlice(elements)
				for i, match := range matches {
					if numGroups == 1 {
						start, end := match[2], match[3]
						if start >= 0 && end >= 0 {
							elements[i] = object.NewSubstring(text[start:end], text)
						} else {
							elements[i] = object.NewString("")
						}
						continue
					}

					groupElements := make([]object.Object, numGroups)
					object.TrackSlice(groupElements)
					for j := 0; j < numGroups; j++ {
						start, end := match[(j+1)*2], match[(j+1)*2+1]
						if start >= 0 && end >= 0 {
							groupElements[j] = object.NewSubstring(text[start:end], text)
						} else {
							groupElements[j] = object.NewString("")
						}
					}
					elements[i] = &object.Tuple{Elements: groupElements}
				}
				return &object.List{Elements: elements}
			},
			HelpText: `findall(string) - Find all matches

Returns a list of all substrings that match the regex pattern.
If the pattern contains capturing groups, returns a list of tuples containing the groups.
If there is one capturing group, returns a list of strings for that group.
If there are no capturing groups, returns a list of strings for the full matches.`,
		},
		"finditer": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("finditer", kwargs); err != nil {
					return err
				}
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				if args[1].Type() != object.STRING_OBJ {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}
				regex := args[0].(*object.Instance)
				pattern := regex.Field("pattern").(*object.String).StringValue()
				text, _ := args[1].AsString()

				re, err := GetCompiledRegex(pattern)
				if err != nil {
					return reCompileError(err)
				}

				allMatches := re.FindAllStringSubmatchIndex(text, -1)
				elements := make([]object.Object, len(allMatches))
				object.TrackSlice(elements)
				conv := &runeOffsets{text: text}
				for i, match := range allMatches {
					elements[i] = createMatchInstance(text, match, re.SubexpNames(), conv)
				}
				return &object.List{Elements: elements}
			},
			HelpText: `finditer(string) - Find all matches as Match objects

Returns a list of Match objects for all matches of the regex pattern in the string.`,
		},
	},
}

// Match class definition
var MatchClass = &object.Class{
	Name: "Match",
	Methods: map[string]object.Object{
		"group": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("group", kwargs); err != nil {
					return err
				}
				match := args[0].(*object.Instance)
				groups := match.Field("_groups").(*object.List).Elements
				if len(args) == 1 {
					return groups[0]
				}
				// group(a, b, ...) returns a tuple of the groups, as in Python.
				out := make([]object.Object, len(args)-1)
				for i, g := range args[1:] {
					n, errObj := matchGroupIndex(match, g)
					if errObj != nil {
						return errObj
					}
					out[i] = groups[n]
				}
				if len(out) == 1 {
					return out[0]
				}
				return &object.Tuple{Elements: out}
			},
			HelpText: `group([n, ...]) - Return the nth matched group (0 = full match, None for a group that did not take part), or the group captured under a (?P<name>...) name; with several arguments, a tuple`,
		},
		"groupdict": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("groupdict", kwargs); err != nil {
					return err
				}
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				match := args[0].(*object.Instance)
				groups := match.Field("_groups").(*object.List).Elements
				names := match.Field("_group_names").(*object.List).Elements
				result := object.NewDict()
				for i, n := range names {
					ns, ok := n.(*object.String)
					if !ok || ns.StringValue() == "" || i >= len(groups) {
						continue
					}
					result.SetByString(ns.StringValue(), groups[i])
				}
				return result
			},
			HelpText: `groupdict() - Return a dict of named groups to their matched text`,
		},
		"groups": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("groups", kwargs); err != nil {
					return err
				}
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				match := args[0].(*object.Instance)
				groups := match.Field("_groups").(*object.List).Elements
				if len(groups) <= 1 {
					return &object.Tuple{Elements: []object.Object{}}
				}
				elements := make([]object.Object, len(groups)-1)
				object.TrackSlice(elements)
				for i := 1; i < len(groups); i++ {
					elements[i-1] = groups[i]
				}
				return &object.Tuple{Elements: elements}
			},
			HelpText: `groups() - Return tuple of all matched groups (excluding group 0)`,
		},
		"start": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				start, _, errObj := matchSpanArgs("start", kwargs, args)
				if errObj != nil {
					return errObj
				}
				return start
			},
			HelpText: `start(n=0) - Return the start position of the nth group (-1 if it did not take part)`,
		},
		"end": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				_, end, errObj := matchSpanArgs("end", kwargs, args)
				if errObj != nil {
					return errObj
				}
				return end
			},
			HelpText: `end(n=0) - Return the end position of the nth group (-1 if it did not take part)`,
		},
		"span": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				start, end, errObj := matchSpanArgs("span", kwargs, args)
				if errObj != nil {
					return errObj
				}
				return &object.Tuple{Elements: []object.Object{start, end}}
			},
			HelpText: `span(n=0) - Return (start, end) tuple for nth group ((-1, -1) if it did not take part)`,
		},
	},
}

// Helper function to create a Regex instance
func createRegexInstance(pattern string, flags int64) *object.Instance {
	return object.NewInstanceWithFields(RegexClass, map[string]object.Object{
		"pattern": object.NewString(pattern),
		"flags":   object.NewInteger(flags),
	})
}

// matchGroupIndex resolves a group number or (?P<name>...) name of a match
// to its index, with Python's IndexError for one the pattern lacks.
func matchGroupIndex(match *object.Instance, g object.Object) (int, object.Object) {
	groups := match.Field("_groups").(*object.List).Elements
	switch g := g.(type) {
	case *object.Integer:
		if n := g.IntValue(); n >= 0 && n < int64(len(groups)) {
			return int(n), nil
		}
	case *object.String:
		names := match.Field("_group_names").(*object.List).Elements
		for i, n := range names {
			if ns, ok := n.(*object.String); ok && ns.StringValue() != "" && ns.StringValue() == g.StringValue() {
				return i, nil
			}
		}
	}
	return 0, &object.Exception{Message: "no such group", ExceptionType: object.ExceptionTypeIndexError, Raised: true}
}

// matchSpanArgs returns the start and end, in characters, of the group a
// start(), end() or span() call names (group 0 by default).
func matchSpanArgs(name string, kwargs object.Kwargs, args []object.Object) (object.Object, object.Object, object.Object) {
	if err := errors.NoKwargs(name, kwargs); err != nil {
		return nil, nil, err
	}
	if len(args) > 2 {
		return nil, nil, errors.NewTypeErrorTagged("%s() takes at most 1 argument (%d given)", name, len(args)-1)
	}
	match := args[0].(*object.Instance)
	n := 0
	if len(args) == 2 {
		var errObj object.Object
		if n, errObj = matchGroupIndex(match, args[1]); errObj != nil {
			return nil, nil, errObj
		}
	}
	if n == 0 {
		return match.Field("_start"), match.Field("_end"), nil
	}
	spans := match.Field("_spans").(*object.List).Elements
	return spans[2*n], spans[2*n+1], nil
}

// runeOffsets converts byte offsets into a string to Python's character
// offsets. Offsets asked for in increasing order (the matches of finditer
// or sub) are counted on from the last one, so converting all of them costs
// one pass over the text.
type runeOffsets struct {
	text               string
	lastByte, lastRune int
}

func (c *runeOffsets) at(b int) int {
	if b < 0 {
		return -1
	}
	if b < c.lastByte {
		c.lastByte, c.lastRune = 0, 0
	}
	c.lastRune += utf8.RuneCountInString(c.text[c.lastByte:b])
	c.lastByte = b
	return c.lastRune
}

// createMatchInstance creates a Match from the submatch byte indices of a
// match in text, as regexp's Find*SubmatchIndex return them. A group that
// did not take part in the match is None, as in Python, and positions are in
// characters. conv may be shared by the matches of one search, in order.
func createMatchInstance(text string, match []int, groupNames []string, conv *runeOffsets) *object.Instance {
	n := len(match) / 2
	groupObjects := make([]object.Object, n)
	object.TrackSlice(groupObjects)
	var spans []object.Object
	if n > 1 {
		spans = make([]object.Object, len(match))
		object.TrackSlice(spans)
	}
	for i := 0; i < n; i++ {
		s, e := match[2*i], match[2*i+1]
		if s >= 0 && e >= 0 {
			groupObjects[i] = object.NewSubstring(text[s:e], text)
		} else {
			groupObjects[i] = &object.Null{}
		}
		if spans != nil && i > 0 {
			spans[2*i] = object.NewInteger(int64(conv.at(s)))
			spans[2*i+1] = object.NewInteger(int64(conv.at(e)))
		}
	}
	nameObjects := make([]object.Object, n)
	object.TrackSlice(nameObjects)
	for i := range nameObjects {
		name := ""
		if i < len(groupNames) {
			name = groupNames[i]
		}
		nameObjects[i] = object.NewString(name)
	}
	start := conv.at(match[0])
	end := conv.at(match[1])
	// The fields carry underscored names: a field named "start" would shadow
	// the start() method on the class, making m.start() an "INTEGER object is
	// not callable" error while m.start silently returned the raw field.
	return object.NewInstanceWithFields(MatchClass, map[string]object.Object{
		"_groups":      &object.List{Elements: groupObjects},
		"_group_names": &object.List{Elements: nameObjects},
		"_spans":       &object.List{Elements: spans},
		"_start":       object.NewInteger(int64(start)),
		"_end":         object.NewInteger(int64(end)),
	})
}

type regexEntry struct {
	pattern  string
	regex    *regexp.Regexp
	element  *list.Element
	lastUsed time.Time
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
		var builder strings.Builder
		builder.Grow(len(prefix) + len(pattern) + 4)
		builder.WriteString("(?")
		builder.WriteString(prefix)
		builder.WriteByte(')')
		builder.WriteString(pattern)
		return builder.String()
	}
	return pattern
}

// GetCompiledRegex retrieves a compiled regex from cache or compiles and caches it
func GetCompiledRegex(pattern string) (*regexp.Regexp, error) {
	globalRegexCache.mu.RLock()
	if entry, ok := globalRegexCache.entries[pattern]; ok {
		// Move to front (most recently used)
		globalRegexCache.mu.RUnlock()
		globalRegexCache.mu.Lock()
		globalRegexCache.lru.MoveToFront(entry.element)
		entry.lastUsed = time.Now()
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

	// Enforce the cap: when full, the least recently used entry goes,
	// recency notwithstanding. Protecting recently used entries here let the
	// cache grow past its maximum without bound under sustained use.
	for len(globalRegexCache.entries) >= globalRegexCache.maxSize {
		if !globalRegexCache.evictOldest() {
			break
		}
	}

	// Add new entry at front
	entry := &regexEntry{
		pattern:  pattern,
		regex:    re,
		lastUsed: time.Now(),
	}
	elem := globalRegexCache.lru.PushFront(entry)
	entry.element = elem
	globalRegexCache.entries[pattern] = entry

	return re, nil
}

func (c *regexCache) evictOldest() bool {
	elem := c.lru.Back()
	if elem == nil {
		return false
	}

	entry := elem.Value.(*regexEntry)
	c.lru.Remove(elem)
	delete(c.entries, entry.pattern)
	return true
}

// Helper to extract the optional flags argument, given positionally at
// flagsIndex or as the flags= keyword (as Python's re functions accept).
func getFlags(args []object.Object, kwargs object.Kwargs, flagsIndex int) (int64, error) {
	var flagsObj object.Object
	if len(args) > flagsIndex {
		flagsObj = args[flagsIndex]
		if kwargs.Has("flags") {
			return 0, fmt.Errorf("argument for flags given by name ('flags') and position (%d)", flagsIndex+1)
		}
	} else if v := kwargs.Get("flags"); v != nil {
		flagsObj = v
	}
	if flagsObj == nil {
		return 0, nil
	}
	if flagsObj.Type() != object.INTEGER_OBJ {
		return 0, fmt.Errorf("flags must be an integer")
	}
	val, _ := flagsObj.AsInt()
	return val, nil
}

var ReLibrary = object.NewLibrary(ReLibraryName, map[string]*object.Builtin{
	"match": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("match", kwargs, "flags"); err != nil {
					return err
				}
			}
			if len(args) < 2 || len(args) > 3 {
				return errors.NewTypeErrorTagged("match() takes 2 or 3 arguments (%d given)", len(args))
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern, _ := args[0].AsString()
			text, _ := args[1].AsString()

			flags, err := getFlags(args, kwargs, 2)
			if err != nil {
				return errors.NewTypeErrorTagged("%s", err.Error())
			}
			pattern = applyFlags(pattern, flags)

			re, err := GetCompiledRegex(pattern)
			if err != nil {
				return reCompileError(err)
			}

			// Check if pattern matches at the beginning of text
			match := re.FindStringSubmatchIndex(text)
			if match == nil || match[0] != 0 {
				return &object.Null{}
			}

			return createMatchInstance(text, match, re.SubexpNames(), &runeOffsets{text: text})
		},
		HelpText: `match(pattern, string, flags=0) - Match pattern at start of string

Returns a Match object if the pattern matches at the beginning, or None if no match.
Use match.group(0) for the full match, match.group(1) for the first group, etc.

Methods on Match object:
  group(n=0)  - Return the nth matched group (0 = full match)
  groups()    - Return tuple of all matched groups (excluding group 0)
  start(n=0)  - Return start position of nth group
  end(n=0)    - Return end position of nth group
  span(n=0)   - Return (start, end) tuple for nth group

Flags:
  re.IGNORECASE or re.I - Case-insensitive matching
  re.MULTILINE or re.M  - ^ and $ match at line boundaries
  re.DOTALL or re.S     - . matches newlines`,
	},
	"search": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("search", kwargs, "flags"); err != nil {
					return err
				}
			}
			if len(args) < 2 || len(args) > 3 {
				return errors.NewTypeErrorTagged("search() takes 2 or 3 arguments (%d given)", len(args))
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern, _ := args[0].AsString()
			text, _ := args[1].AsString()

			flags, err := getFlags(args, kwargs, 2)
			if err != nil {
				return errors.NewTypeErrorTagged("%s", err.Error())
			}
			pattern = applyFlags(pattern, flags)

			re, err := GetCompiledRegex(pattern)
			if err != nil {
				return reCompileError(err)
			}

			match := re.FindStringSubmatchIndex(text)
			if match == nil {
				return &object.Null{}
			}

			return createMatchInstance(text, match, re.SubexpNames(), &runeOffsets{text: text})
		},
		HelpText: `search(pattern, string, flags=0) - Search for pattern anywhere in string

Returns a Match object for the first match, or None if no match found.
Use match.group(0) for the full match, match.group(1) for the first group, etc.

Methods on Match object:
  group(n=0)  - Return the nth matched group (0 = full match)
  groups()    - Return tuple of all matched groups (excluding group 0)
  start(n=0)  - Return start position of nth group
  end(n=0)    - Return end position of nth group
  span(n=0)   - Return (start, end) tuple for nth group

Flags:
  re.IGNORECASE or re.I - Case-insensitive matching
  re.MULTILINE or re.M  - ^ and $ match at line boundaries
  re.DOTALL or re.S     - . matches newlines`,
	},
	"findall": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("findall", kwargs, "flags"); err != nil {
					return err
				}
			}
			if len(args) < 2 || len(args) > 3 {
				return errors.NewTypeErrorTagged("findall() takes 2 or 3 arguments (%d given)", len(args))
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern, _ := args[0].AsString()
			text, _ := args[1].AsString()

			flags, err := getFlags(args, kwargs, 2)
			if err != nil {
				return errors.NewTypeErrorTagged("%s", err.Error())
			}
			pattern = applyFlags(pattern, flags)

			re, err := GetCompiledRegex(pattern)
			if err != nil {
				return reCompileError(err)
			}

			matches := re.FindAllStringSubmatch(text, -1)
			elements := make([]object.Object, len(matches))
			object.TrackSlice(elements)
			numGroups := re.NumSubexp()
			for i, match := range matches {
				if numGroups == 0 {
					elements[i] = object.NewSubstring(match[0], text)
				} else if numGroups == 1 {
					elements[i] = object.NewSubstring(match[1], text)
				} else {
					groupElements := make([]object.Object, numGroups)
					object.TrackSlice(groupElements)
					for j := 1; j <= numGroups; j++ {
						groupElements[j-1] = object.NewSubstring(match[j], text)
					}
					elements[i] = &object.Tuple{Elements: groupElements}
				}
			}
			return &object.List{Elements: elements}
		},
		HelpText: `findall(pattern, string, flags=0) - Find all matches

Returns a list of all substrings that match the regex pattern.
If the pattern contains capturing groups, returns a list of tuples containing the groups.
If there is one capturing group, returns a list of strings for that group.
If there are no capturing groups, returns a list of strings for the full matches.

Flags:
  re.IGNORECASE or re.I - Case-insensitive matching
  re.MULTILINE or re.M  - ^ and $ match at line boundaries
  re.DOTALL or re.S     - . matches newlines`,
	},
	"finditer": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("finditer", kwargs, "flags"); err != nil {
					return err
				}
			}
			if len(args) < 2 || len(args) > 3 {
				return errors.NewTypeErrorTagged("finditer() takes 2 or 3 arguments (%d given)", len(args))
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern, _ := args[0].AsString()
			text, _ := args[1].AsString()

			flags, err := getFlags(args, kwargs, 2)
			if err != nil {
				return errors.NewTypeErrorTagged("%s", err.Error())
			}
			pattern = applyFlags(pattern, flags)

			re, err := GetCompiledRegex(pattern)
			if err != nil {
				return reCompileError(err)
			}

			allMatches := re.FindAllStringSubmatchIndex(text, -1)
			elements := make([]object.Object, len(allMatches))
			object.TrackSlice(elements)
			conv := &runeOffsets{text: text}
			for i, match := range allMatches {
				elements[i] = createMatchInstance(text, match, re.SubexpNames(), conv)
			}
			return &object.List{Elements: elements}
		},
		HelpText: `finditer(pattern, string, flags=0) - Find all matches as Match objects

Returns a list of Match objects for all matches of the regex pattern in the string.

Flags:
  re.IGNORECASE or re.I - Case-insensitive matching
  re.MULTILINE or re.M  - ^ and $ match at line boundaries
  re.DOTALL or re.S     - . matches newlines`,
	},
	"sub": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("sub", kwargs, "count", "flags"); err != nil {
					return err
				}
			}
			if len(args) < 3 || len(args) > 5 {
				return errors.NewTypeErrorTagged("sub() takes 3 to 5 arguments (%d given)", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			if args[2].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", args[2].Type().String())
			}
			pattern, _ := args[0].AsString()
			text, _ := args[2].AsString()

			// Check if replacement is a string or function
			isFunc := false
			var replacementStr string
			var replacementFunc object.Object

			switch args[1].Type() {
			case object.STRING_OBJ:
				replacementStr, _ = args[1].AsString()
			case object.FUNCTION_OBJ, object.BUILTIN_OBJ, object.LAMBDA_OBJ:
				isFunc = true
				replacementFunc = args[1]
			default:
				return errors.NewTypeError("STRING or FUNCTION", args[1].Type().String())
			}

			// count parameter (optional, position 3 or count= kwarg);
			// 0 or omitted means replace all, as in Python.
			count := -1
			countObj := object.Object(nil)
			if len(args) > 3 {
				countObj = args[3]
			} else if v := kwargs.Get("count"); v != nil {
				countObj = v
			}
			if countObj != nil {
				if countObj.Type() != object.INTEGER_OBJ {
					return errors.NewTypeErrorTagged("count must be an integer")
				}
				val, _ := countObj.AsInt()
				count = int(val)
				if count == 0 {
					count = -1
				}
			}

			// flags parameter (optional, position 4)
			flags, err := getFlags(args, kwargs, 4)
			if err != nil {
				return errors.NewTypeErrorTagged("%s", err.Error())
			}
			pattern = applyFlags(pattern, flags)

			re, err := GetCompiledRegex(pattern)
			if err != nil {
				return reCompileError(err)
			}

			var result string
			if isFunc {
				// Function replacement - find all matches in the original text
				replaced := 0
				allMatches := re.FindAllStringSubmatchIndex(text, -1)

				if len(allMatches) == 0 {
					return object.NewString(text)
				}

				// Build result by replacing matches. Every callback is charged
				// to a WorkMeter, so a long run of callbacks (each of which may
				// be a lambda with no loop of its own to check) honours the
				// call's cancellation, and the result is held to the value
				// limit as it grows rather than once it is built.
				var resultBuilder []byte
				lastEnd := 0
				meter := libutil.NewWorkMeter(ctx)
				grownTo := 0
				conv := &runeOffsets{text: text}

				for _, match := range allMatches {
					if count > 0 && replaced >= count {
						break
					}
					if errObj := meter.Add(256); errObj != nil {
						return errObj
					}

					// Add text before match
					resultBuilder = append(resultBuilder, text[lastEnd:match[0]]...)

					matchObj := createMatchInstance(text, match, re.SubexpNames(), conv)

					// Call the replacement function
					var resultObj object.Object
					if replacementFunc.Type() == object.BUILTIN_OBJ {
						builtin := replacementFunc.(*object.Builtin)
						resultObj = builtin.Fn(ctx, object.Kwargs{}, matchObj)
					} else {
						eval := evaliface.FromContext(ctx)
						if eval == nil {
							return errors.NewError("evaluator not available in context")
						}
						env := object.NewEnvironment()
						if ctxEnv, ok := ctx.Value("scriptling-env").(*object.Environment); ok {
							env = ctxEnv
						}
						resultObj = eval.CallObjectFunction(ctx, replacementFunc, []object.Object{matchObj}, nil, env)
					}

					// A raised exception (or internal error) from the replacement
					// callback must propagate — never be coerced to a string and
					// spliced into the result (silent data corruption).
					if resultObj.Type() == object.ERROR_OBJ || resultObj.Type() == object.EXCEPTION_OBJ {
						return resultObj
					}

					// Convert result to string
					if resultStr, err := resultObj.AsString(); err == nil {
						resultBuilder = append(resultBuilder, resultStr...)
					} else {
						// If conversion fails, keep original match
						resultBuilder = append(resultBuilder, text[match[0]:match[1]]...)
					}

					if len(resultBuilder) > grownTo {
						// Checked as the buffer grows, which it does by doubling.
						grownTo = cap(resultBuilder)
						if errObj := object.CheckAllocFor("substituted string", int64(len(resultBuilder))); errObj != nil {
							return errObj
						}
					}

					lastEnd = match[1]
					replaced++
				}

				// Add remaining text
				resultBuilder = append(resultBuilder, text[lastEnd:]...)
				result = string(resultBuilder)
			} else {
				template, errObj := pythonReplToGo(replacementStr, re)
				if errObj != nil {
					return errObj
				}
				if errObj := checkSubSize(re, text, template, count); errObj != nil {
					return errObj
				}
				if count < 0 {
					result = re.ReplaceAllString(text, template)
				} else {
					// Replace only the first count occurrences, expanding
					// each against the whole text as ReplaceAllString does.
					var out []byte
					last := 0
					for _, m := range re.FindAllStringSubmatchIndex(text, count) {
						out = append(out, text[last:m[0]]...)
						out = re.ExpandString(out, template, text, m)
						last = m[1]
					}
					result = string(append(out, text[last:]...))
				}
			}
			return object.NewString(result)
		},
		HelpText: `sub(pattern, repl, string, count=0, flags=0) - Replace matches

Replaces occurrences of the regex pattern in the string with the replacement.
The replacement can be either a string or a function that takes a Match object and returns a string.
If count is 0 (default), all occurrences are replaced.
If count > 0, only the first count occurrences are replaced.

Flags:
  re.IGNORECASE or re.I - Case-insensitive matching
  re.MULTILINE or re.M  - ^ and $ match at line boundaries
  re.DOTALL or re.S     - . matches newlines`,
	},
	"subn": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("subn", kwargs, "count", "flags"); err != nil {
					return err
				}
			}
			// subn(pattern, repl, string[, count[, flags]]) returns
			// (new_string, number_of_substitutions), like Python.
			result := reSubBuiltin.Fn(ctx, kwargs, args...)
			if object.IsError(result) || result.Type() == object.EXCEPTION_OBJ {
				return result
			}
			newText, _ := result.(*object.String)

			pattern, _ := args[0].AsString()
			text, _ := args[2].AsString()
			count := int64(-1)
			countObj := object.Object(nil)
			if len(args) > 3 {
				countObj = args[3]
			} else if v := kwargs.Get("count"); v != nil {
				countObj = v
			}
			if countObj != nil {
				if v, err := countObj.AsInt(); err == nil {
					count = v
				}
			}
			if flags, err := getFlags(args, kwargs, 4); err == nil {
				pattern = applyFlags(pattern, flags)
			}
			re, err := GetCompiledRegex(pattern)
			if err != nil {
				return reCompileError(err)
			}
			all := re.FindAllStringIndex(text, int(count))
			return &object.Tuple{Elements: []object.Object{newText, object.NewInteger(int64(len(all)))}}
		},
		HelpText: `subn(pattern, repl, string, count=0) - Replace matches, returning (new_string, n)

Like sub(), but returns a tuple of the modified string and the number of
substitutions made.`,
	},
	"split": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("split", kwargs, "maxsplit", "flags"); err != nil {
					return err
				}
			}
			if len(args) < 2 || len(args) > 4 {
				return errors.NewTypeErrorTagged("split() takes 2 to 4 arguments (%d given)", len(args))
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern, _ := args[0].AsString()
			text, _ := args[1].AsString()

			// maxsplit parameter (optional, position 2 or maxsplit= kwarg);
			// 0 or omitted means no limit, as in Python.
			maxsplit := -1
			maxObj := object.Object(nil)
			if len(args) > 2 {
				maxObj = args[2]
			} else if v := kwargs.Get("maxsplit"); v != nil {
				maxObj = v
			}
			if maxObj != nil {
				if maxObj.Type() != object.INTEGER_OBJ {
					return errors.NewTypeErrorTagged("maxsplit must be an integer")
				}
				val, _ := maxObj.AsInt()
				maxsplit = int(val)
				if maxsplit == 0 {
					maxsplit = -1
				}
			}

			// flags parameter (optional, position 3)
			flags, err := getFlags(args, kwargs, 3)
			if err != nil {
				return errors.NewTypeErrorTagged("%s", err.Error())
			}
			pattern = applyFlags(pattern, flags)

			re, err := GetCompiledRegex(pattern)
			if err != nil {
				return reCompileError(err)
			}

			// The text between matches, with each match's groups in
			// between when the pattern has any (None for a group that did
			// not take part), as in Python: re.split(r'(,)', 'a,b') is
			// ['a', ',', 'b']. An empty match splits too: re.split('', 'ab')
			// is ['', 'a', 'b', ''].
			matches := re.FindAllStringSubmatchIndex(text, maxsplit)
			elements := make([]object.Object, 0, len(matches)*(1+re.NumSubexp())+1)
			last := 0
			for _, m := range matches {
				elements = append(elements, object.NewSubstring(text[last:m[0]], text))
				for g := 1; g <= re.NumSubexp(); g++ {
					if s, e := m[2*g], m[2*g+1]; s >= 0 {
						elements = append(elements, object.NewSubstring(text[s:e], text))
					} else {
						elements = append(elements, &object.Null{})
					}
				}
				last = m[1]
			}
			elements = append(elements, object.NewSubstring(text[last:], text))
			object.TrackSlice(elements)
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
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("compile", kwargs, "flags"); err != nil {
					return err
				}
			}
			if len(args) < 1 || len(args) > 2 {
				return errors.NewTypeErrorTagged("compile() takes 1 or 2 arguments (%d given)", len(args))
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			pattern, _ := args[0].AsString()

			flags, err := getFlags(args, kwargs, 1)
			if err != nil {
				return errors.NewTypeErrorTagged("%s", err.Error())
			}
			pattern = applyFlags(pattern, flags)

			// Try to compile to validate the pattern
			_, err = GetCompiledRegex(pattern)
			if err != nil {
				return reCompileError(err)
			}

			// Return the compiled regex object
			return createRegexInstance(pattern, flags)
		},
		HelpText: `compile(pattern, flags=0) - Compile regex pattern

Validates and caches a regex pattern for later use. Returns the pattern if valid.

Flags:
  re.IGNORECASE or re.I - Case-insensitive matching
  re.MULTILINE or re.M  - ^ and $ match at line boundaries
  re.DOTALL or re.S     - . matches newlines`,
	},
	"escape": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("escape", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			text, _ := args[0].AsString()

			escaped := regexp.QuoteMeta(text)
			return object.NewString(escaped)
		},
		HelpText: `escape(pattern) - Escape special regex characters

Returns a string with all special regex characters escaped.`,
	},
	"fullmatch": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("fullmatch", kwargs, "flags"); err != nil {
					return err
				}
			}
			if len(args) < 2 || len(args) > 3 {
				return errors.NewTypeErrorTagged("fullmatch() takes 2 or 3 arguments (%d given)", len(args))
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern, _ := args[0].AsString()
			text, _ := args[1].AsString()

			flags, err := getFlags(args, kwargs, 2)
			if err != nil {
				return errors.NewTypeErrorTagged("%s", err.Error())
			}
			pattern = applyFlags(pattern, flags)

			re, err := GetCompiledRegex(pattern)
			if err != nil {
				return reCompileError(err)
			}

			// Check if the entire string matches
			match := re.FindStringSubmatchIndex(text)
			if match == nil || match[0] != 0 || match[1] != len(text) {
				return &object.Null{}
			}

			// Build groups from submatch indices (same as match/search)

			return createMatchInstance(text, match, re.SubexpNames(), &runeOffsets{text: text})
		},
		HelpText: `fullmatch(pattern, string, flags=0) - Match entire string

Returns a Match object if the regex pattern matches the entire string, or None if no match.

Flags:
  re.IGNORECASE or re.I - Case-insensitive matching
  re.MULTILINE or re.M  - ^ and $ match at line boundaries
  re.DOTALL or re.S     - . matches newlines`,
	},
}, map[string]object.Object{
	// Flag constants - matching Python's re module values
	"IGNORECASE": object.NewInteger(RE_IGNORECASE),
	"I":          object.NewInteger(RE_IGNORECASE),
	"MULTILINE":  object.NewInteger(RE_MULTILINE),
	"M":          object.NewInteger(RE_MULTILINE),
	"DOTALL":     object.NewInteger(RE_DOTALL),
	"S":          object.NewInteger(RE_DOTALL),
	// re.error and its Python 3.13 name re.PatternError, raised for a bad
	// pattern or replacement template.
	"error":        reErrorConstructor,
	"PatternError": reErrorConstructor,
}, "Regular expression library")

var reErrorConstructor = object.RegisterExceptionClass(&object.Builtin{
	Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if kwargs.Len() > 0 {
			if err := errors.CheckKwargs("error", kwargs, "pattern", "pos"); err != nil {
				return err
			}
		}
		message := ""
		if len(args) > 0 {
			if str, err := args[0].AsString(); err == nil {
				message = str
			} else {
				message = args[0].Inspect()
			}
		}
		return &object.Exception{Message: message, ExceptionType: "PatternError", TypeChain: []string{"PatternError", "error"}}
	},
	HelpText: `error(msg) - The exception raised for a bad regular expression (also re.PatternError)`,
	Repr:     "<class 're.PatternError'>",
}, "PatternError")

// checkSubSize holds a substitution with a template replacement to the
// output limit before it is made: each match expands the template, so a
// long replacement over many matches can make a result far larger than
// either string. With no limit set it does nothing. count < 0 replaces all.
func checkSubSize(re *regexp.Regexp, text, template string, count int) object.Object {
	limit := object.OutputLimit()
	if limit <= 0 {
		return nil
	}
	matches := re.FindAllStringSubmatchIndex(text, count)
	size := int64(len(text))
	var scratch []byte
	for _, m := range matches {
		scratch = re.ExpandString(scratch[:0], template, text, m)
		size += int64(len(scratch)) - int64(m[1]-m[0])
		if size > limit {
			return object.NewOutputTooLargeError("substituted string", limit)
		}
	}
	return nil
}

// pythonReplToGo converts a Python replacement template to Go's regexp
// template syntax, raising as Python does for a bad one: \1..\99 and
// \g<n> / \g<name> become ${n} / ${name} (an error for a group the pattern
// lacks), a $ not starting a Go-style $1 or ${name} becomes a literal $,
// \\ is a backslash, \n, \t and the other character escapes become their
// characters, an unknown ASCII-letter escape is an error, and any other
// backslash stays as it is.
func pythonReplToGo(repl string, re *regexp.Regexp) (string, object.Object) {
	if strings.IndexByte(repl, '\\') < 0 && strings.IndexByte(repl, '$') < 0 {
		return repl, nil
	}
	groups := re.NumSubexp()
	var out strings.Builder
	for i := 0; i < len(repl); i++ {
		c := repl[i]
		if c == '$' {
			// $1 and ${name} are kept as Go-style backreferences, which
			// scriptling has long accepted; any other $ is literal, as in
			// Python.
			if i+1 < len(repl) && (repl[i+1] == '{' || repl[i+1] >= '0' && repl[i+1] <= '9') {
				out.WriteByte('$')
			} else {
				out.WriteString("$$")
			}
			continue
		}
		if c != '\\' || i+1 >= len(repl) {
			out.WriteByte(c)
			continue
		}
		next := repl[i+1]
		switch {
		case next == '\\':
			out.WriteByte('\\')
			i++
		case next >= '1' && next <= '9':
			j := i + 2
			if j < len(repl) && repl[j] >= '0' && repl[j] <= '9' {
				j++
			}
			n := 0
			for _, d := range repl[i+1 : j] {
				n = n*10 + int(d-'0')
			}
			if n > groups {
				return "", reError(fmt.Sprintf("invalid group reference %d at position %d", n, i+1))
			}
			fmt.Fprintf(&out, "${%d}", n)
			i = j - 1
		case next == 'g':
			end := -1
			if i+2 < len(repl) && repl[i+2] == '<' {
				end = strings.IndexByte(repl[i+3:], '>')
			}
			if end <= 0 {
				return "", reError(fmt.Sprintf("missing group name at position %d", i+2))
			}
			name := repl[i+3 : i+3+end]
			if n, err := strconv.Atoi(name); err == nil {
				if n < 0 || n > groups {
					return "", reError(fmt.Sprintf("invalid group reference %d at position %d", n, i+3))
				}
			} else if re.SubexpIndex(name) < 0 {
				return "", &object.Exception{Message: "unknown group name '" + name + "'", ExceptionType: object.ExceptionTypeIndexError, Raised: true}
			}
			out.WriteString("${" + name + "}")
			i = i + 3 + end
		default:
			if esc, ok := replEscapes[next]; ok {
				out.WriteByte(esc)
				i++
			} else if next == '0' {
				out.WriteByte(0)
				i++
			} else if next < utf8.RuneSelf && ('a' <= next && next <= 'z' || 'A' <= next && next <= 'Z') {
				return "", reError(fmt.Sprintf("bad escape \\%c at position %d", next, i))
			} else {
				out.WriteByte(c)
			}
		}
	}
	return out.String(), nil
}

// replEscapes are the character escapes a Python replacement template
// accepts.
var replEscapes = map[byte]byte{'n': '\n', 't': '\t', 'r': '\r', 'f': '\f', 'v': '\v', 'a': '\a', 'b': '\b'}

// reError is re.error (re.PatternError), raised for a bad pattern or
// replacement template. As in Python it derives from Exception only;
// `except re.error:` and `except re.PatternError:` both catch it.
func reError(msg string) object.Object {
	return &object.Exception{Message: msg, ExceptionType: "PatternError", TypeChain: []string{"PatternError", "error"}, Raised: true}
}

// reCompileError is the re.error for a pattern regexp cannot compile.
func reCompileError(err error) object.Object {
	return reError("regex compile error: " + err.Error())
}

// reSubBuiltin is wired in init() so subn can reuse sub's implementation
// without an initialization cycle in the ReLibrary map literal.
var reSubBuiltin *object.Builtin

func init() {
	reSubBuiltin = ReLibrary.Functions()["sub"]
}
