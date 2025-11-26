package stdlib

import (
	"container/list"
	"context"
	"regexp"
	"sync"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
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

var reLibrary = object.NewLibrary(map[string]*object.Builtin{
	"match": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern, _ := args[0].AsString()
			text, _ := args[1].AsString()

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
	},
	"find": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern, _ := args[0].AsString()
			text, _ := args[1].AsString()

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
	},
	"findall": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern, _ := args[0].AsString()
			text, _ := args[1].AsString()

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
	},
	"replace": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 3 {
				return errors.NewArgumentError(len(args), 3)
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ || args[2].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern, _ := args[0].AsString()
			text, _ := args[1].AsString()
			replacement, _ := args[2].AsString()

			re, err := getCompiledRegex(pattern)
			if err != nil {
				return errors.NewError("regex compile error: %s", err.Error())
			}

			result := re.ReplaceAllString(text, replacement)
			return &object.String{Value: result}
		},
	},
	"split": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern, _ := args[0].AsString()
			text, _ := args[1].AsString()

			re, err := getCompiledRegex(pattern)
			if err != nil {
				return errors.NewError("regex compile error: %s", err.Error())
			}

			parts := re.Split(text, -1)
			elements := make([]object.Object, len(parts))
			for i, part := range parts {
				elements[i] = &object.String{Value: part}
			}
			return &object.List{Elements: elements}
		},
	},
	"search": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern, _ := args[0].AsString()
			text, _ := args[1].AsString()

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
	},
	"compile": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			pattern, _ := args[0].AsString()

			// Try to compile to validate the pattern
			_, err := getCompiledRegex(pattern)
			if err != nil {
				return errors.NewError("regex compile error: %s", err.Error())
			}

			// Return the pattern string as a compiled "object"
			return &object.String{Value: pattern}
		},
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
	},
	"fullmatch": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) != 2 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			pattern, _ := args[0].AsString()
			text, _ := args[1].AsString()

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
	},
})

func ReLibrary() *object.Library {
	return reLibrary
}
