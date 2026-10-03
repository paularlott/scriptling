package stdlib

import (
	"context"
	"strings"
	"unicode"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var TextwrapLibrary = object.NewLibrary(TextwrapLibraryName, map[string]*object.Builtin{
	"wrap": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			text, w, err := textwrapArgs("wrap", args, kwargs)
			if err != nil {
				return err
			}
			lines, err := w.wrap(text)
			if err != nil {
				return err
			}
			elements := make([]object.Object, len(lines))
			for i, line := range lines {
				elements[i] = object.NewString(line)
			}
			return &object.List{Elements: elements}
		},
		HelpText: `wrap(text, width=70, **kwargs) - Wrap a single paragraph of text

Parameters (as Python's textwrap.TextWrapper):
  text                 - Text to wrap
  width                - Maximum line width (default 70)
  initial_indent       - String prepended to the first line (default "")
  subsequent_indent    - String prepended to all other lines (default "")
  expand_tabs          - Expand tabs to spaces first (default True)
  tabsize              - Tab size used by expand_tabs (default 8)
  replace_whitespace   - Replace each whitespace char with a space (default True)
  fix_sentence_endings - Two spaces after sentence endings (default False)
  break_long_words     - Break words longer than width (default True)
  drop_whitespace      - Drop whitespace at line starts/ends (default True)
  break_on_hyphens     - Allow breaks after hyphens in words (default True)
  max_lines            - Truncate output to at most this many lines (default None)
  placeholder          - Appended to truncated output (default " [...]")

Returns: List of lines

Example:
  import textwrap
  lines = textwrap.wrap("Hello world, this is a long line", 10)`,
	},

	"fill": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			text, w, err := textwrapArgs("fill", args, kwargs)
			if err != nil {
				return err
			}
			lines, err := w.wrap(text)
			if err != nil {
				return err
			}
			return object.NewString(strings.Join(lines, "\n"))
		},
		HelpText: `fill(text, width=70, **kwargs) - Wrap text and return a single string

Accepts the same keyword options as wrap().

Returns: Wrapped text as single string with newlines

Example:
  import textwrap
  result = textwrap.fill("Hello world, this is a long line", 10)`,
	},

	"dedent": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) != 1 {
				return errors.NewError("dedent() requires exactly 1 argument")
			}

			text, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			return object.NewString(dedentText(text.StringValue()))
		},
		HelpText: `dedent(text) - Remove common leading whitespace from all lines

Parameters:
  text - Text to dedent

Returns: Dedented text

Example:
  import textwrap
  text = """
      Hello
      World
  """
  result = textwrap.dedent(text)  # Lines start at column 0`,
	},

	"indent": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 2 {
				return errors.NewError("indent() requires at least 2 arguments")
			}

			text, ok := args[0].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}

			prefix, ok := args[1].(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", args[1].Type().String())
			}

			for _, k := range kwargs.Keys() {
				if k != "predicate" {
					return newArgTypeError("indent() got an unexpected keyword argument '%s'", k)
				}
			}
			var predicate object.Object
			if len(args) >= 3 {
				predicate = args[2]
			} else if p := kwargs.Get("predicate"); p != nil {
				predicate = p
			}
			if _, isNull := predicate.(*object.Null); isNull {
				predicate = nil
			}

			// Split keeping line endings, like str.splitlines(True).
			var b strings.Builder
			for _, line := range strings.SplitAfter(text.StringValue(), "\n") {
				if line == "" {
					continue
				}
				add := false
				if predicate != nil {
					res := callCallable(ctx, predicate, object.NewString(line))
					if object.IsError(res) || res.Type() == object.EXCEPTION_OBJ {
						return res
					}
					add = isTruthy(res)
				} else {
					// Default: indent lines that are not whitespace-only.
					add = strings.TrimSpace(line) != ""
				}
				if add {
					b.WriteString(prefix.StringValue())
				}
				b.WriteString(line)
			}
			return object.NewString(b.String())
		},
		HelpText: `indent(text, prefix, predicate=None) - Add prefix to lines

Parameters:
  text      - Text to indent
  prefix    - String to add to beginning of each line
  predicate - Optional callable; only lines for which it returns true are
              indented (default: lines that are not whitespace-only)

Returns: Indented text

Example:
  import textwrap
  result = textwrap.indent("Hello\nWorld", "  ")  # "  Hello\n  World"`,
	},

	"shorten": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// Python: TextWrapper(width=width, max_lines=1, **kwargs).fill(
			// " ".join(text.strip().split())).
			if kwargs.Has("max_lines") {
				return newArgTypeError("shorten() got multiple values for keyword argument 'max_lines'")
			}
			if len(args) < 2 && !kwargs.Has("width") {
				return errors.NewError("shorten() missing required argument: 'width'")
			}
			text, w, err := textwrapArgs("shorten", args, kwargs)
			if err != nil {
				return err
			}
			w.maxLines = 1
			lines, err := w.wrap(strings.Join(strings.Fields(text), " "))
			if err != nil {
				return err
			}
			return object.NewString(strings.Join(lines, "\n"))
		},
		HelpText: `shorten(text, width, **kwargs) - Collapse and truncate text to fit width

Parameters:
  text        - Text to shorten
  width       - Maximum width including placeholder
  placeholder - String to indicate truncation (default " [...]")
  (other wrap() keyword options are also accepted)

Returns: Shortened text

Example:
  import textwrap
  result = textwrap.shorten("Hello World!", 11)  # "Hello [...]"`,
	},
}, nil, "Text wrapping and filling utilities")

// dedentText removes common leading whitespace from all lines, as Python's
// textwrap.dedent: lines consisting solely of whitespace are normalized to
// empty, and the margin is the longest common leading-whitespace string (so
// tabs and spaces are not treated as equal).
func dedentText(text string) string {
	lines := strings.Split(text, "\n")
	margin := ""
	first := true
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" || trimmed == "\r" {
			lines[i] = strings.TrimLeft(line, " \t")
			continue
		}
		indent := line[:len(line)-len(trimmed)]
		if first {
			margin = indent
			first = false
			continue
		}
		if strings.HasPrefix(indent, margin) {
			continue
		}
		if strings.HasPrefix(margin, indent) {
			margin = indent
			continue
		}
		j := 0
		for j < len(margin) && j < len(indent) && margin[j] == indent[j] {
			j++
		}
		margin = margin[:j]
	}
	if margin != "" {
		for i, line := range lines {
			lines[i] = strings.TrimPrefix(line, margin)
		}
	}
	return strings.Join(lines, "\n")
}

// textWrapper mirrors Python's textwrap.TextWrapper options and algorithm.
type textWrapper struct {
	width              int
	initialIndent      string
	subsequentIndent   string
	expandTabs         bool
	tabsize            int
	replaceWhitespace  bool
	fixSentenceEndings bool
	breakLongWords     bool
	dropWhitespace     bool
	breakOnHyphens     bool
	maxLines           int // 0 means None
	placeholder        string
}

// textwrapArgs parses (text, width=70, **options) for wrap/fill/shorten.
// Unknown keywords raise an error, as Python's TextWrapper does.
func textwrapArgs(fname string, args []object.Object, kwargs object.Kwargs) (string, *textWrapper, object.Object) {
	w := &textWrapper{
		width:             70,
		expandTabs:        true,
		tabsize:           8,
		replaceWhitespace: true,
		breakLongWords:    true,
		dropWhitespace:    true,
		breakOnHyphens:    true,
		placeholder:       " [...]",
	}
	if len(args) < 1 {
		return "", nil, errors.NewError("%s() missing required argument: 'text'", fname)
	}
	if len(args) > 2 {
		return "", nil, errors.NewError("%s() takes at most 2 positional arguments (%d given)", fname, len(args))
	}
	text, ok := args[0].(*object.String)
	if !ok {
		return "", nil, errors.NewTypeError("STRING", args[0].Type().String())
	}
	if len(args) == 2 {
		if kwargs.Has("width") {
			return "", nil, newArgTypeError("%s() got multiple values for argument 'width'", fname)
		}
		n, ok := args[1].(*object.Integer)
		if !ok {
			return "", nil, errors.NewTypeError("INTEGER", args[1].Type().String())
		}
		w.width = int(n.IntValue())
	}
	for _, k := range kwargs.Keys() {
		v := kwargs.Get(k)
		intOpt := func(dst *int) object.Object {
			n, ok := v.(*object.Integer)
			if !ok {
				return errors.NewTypeError("INTEGER", v.Type().String())
			}
			*dst = int(n.IntValue())
			return nil
		}
		strOpt := func(dst *string) object.Object {
			s, ok := v.(*object.String)
			if !ok {
				return errors.NewTypeError("STRING", v.Type().String())
			}
			*dst = s.StringValue()
			return nil
		}
		var err object.Object
		switch k {
		case "width":
			err = intOpt(&w.width)
		case "tabsize":
			err = intOpt(&w.tabsize)
		case "max_lines":
			if _, isNull := v.(*object.Null); !isNull {
				err = intOpt(&w.maxLines)
			}
		case "initial_indent":
			err = strOpt(&w.initialIndent)
		case "subsequent_indent":
			err = strOpt(&w.subsequentIndent)
		case "placeholder":
			err = strOpt(&w.placeholder)
		case "expand_tabs":
			w.expandTabs = isTruthy(v)
		case "replace_whitespace":
			w.replaceWhitespace = isTruthy(v)
		case "fix_sentence_endings":
			w.fixSentenceEndings = isTruthy(v)
		case "break_long_words":
			w.breakLongWords = isTruthy(v)
		case "drop_whitespace":
			w.dropWhitespace = isTruthy(v)
		case "break_on_hyphens":
			w.breakOnHyphens = isTruthy(v)
		default:
			err = newArgTypeError("%s() got an unexpected keyword argument '%s'", fname, k)
		}
		if err != nil {
			return "", nil, err
		}
	}
	return text.StringValue(), w, nil
}

func runesLen(s string) int { return len([]rune(s)) }

func isWrapSpace(r rune) bool { return unicode.IsSpace(r) }

func isWrapLetter(r rune) bool { return unicode.IsLetter(r) }

// expandTabsStr mirrors str.expandtabs(tabsize).
func expandTabsStr(s string, tabsize int) string {
	var b strings.Builder
	col := 0
	for _, r := range s {
		switch r {
		case '\t':
			if tabsize > 0 {
				n := tabsize - col%tabsize
				b.WriteString(strings.Repeat(" ", n))
				col += n
			}
		case '\n', '\r':
			b.WriteRune(r)
			col = 0
		default:
			b.WriteRune(r)
			col++
		}
	}
	return b.String()
}

// splitChunks splits text into alternating whitespace and word chunks, as
// TextWrapper._split. With breakOnHyphens, hyphenated words also split after
// a hyphen that joins letters ("well-known" -> "well-", "known").
func (w *textWrapper) splitChunks(text string) []string {
	rs := []rune(text)
	var chunks []string
	i := 0
	for i < len(rs) {
		j := i
		if isWrapSpace(rs[i]) {
			for j < len(rs) && isWrapSpace(rs[j]) {
				j++
			}
			chunks = append(chunks, string(rs[i:j]))
			i = j
			continue
		}
		for j < len(rs) && !isWrapSpace(rs[j]) {
			j++
		}
		word := rs[i:j]
		if w.breakOnHyphens {
			start := 0
			for k := 0; k < len(word); k++ {
				if word[k] != '-' || k == start {
					continue
				}
				// Preceded by two letters, or by letter-hyphen-letter.
				before := (k-start >= 2 && isWrapLetter(word[k-1]) && isWrapLetter(word[k-2])) ||
					(k-start >= 3 && isWrapLetter(word[k-1]) && word[k-2] == '-' && isWrapLetter(word[k-3]))
				// Followed by letter, optional hyphen, letter.
				after := k+2 < len(word) && isWrapLetter(word[k+1]) &&
					(isWrapLetter(word[k+2]) || (word[k+2] == '-' && k+3 < len(word) && isWrapLetter(word[k+3])))
				if before && after {
					chunks = append(chunks, string(word[start:k+1]))
					start = k + 1
				}
			}
			chunks = append(chunks, string(word[start:]))
		} else {
			chunks = append(chunks, string(word))
		}
		i = j
	}
	return chunks
}

// isSentenceEnd mirrors TextWrapper.sentence_end_re: [a-z][.!?]["']?\Z
func isSentenceEnd(chunk string) bool {
	rs := []rune(chunk)
	n := len(rs)
	if n > 0 && (rs[n-1] == '"' || rs[n-1] == '\'') {
		n--
	}
	if n < 2 {
		return false
	}
	p := rs[n-1]
	l := rs[n-2]
	return (p == '.' || p == '!' || p == '?') && l >= 'a' && l <= 'z'
}

// wrap mirrors TextWrapper.wrap.
func (w *textWrapper) wrap(text string) ([]string, object.Object) {
	if w.expandTabs {
		text = expandTabsStr(text, w.tabsize)
	}
	if w.replaceWhitespace {
		text = strings.Map(func(r rune) rune {
			switch r {
			case '\t', '\n', '\v', '\f', '\r':
				return ' '
			}
			return r
		}, text)
	}
	chunks := w.splitChunks(text)
	if w.fixSentenceEndings {
		for i := 0; i < len(chunks)-1; {
			if chunks[i+1] == " " && isSentenceEnd(chunks[i]) {
				chunks[i+1] = "  "
				i += 2
			} else {
				i++
			}
		}
	}
	return w.wrapChunks(chunks)
}

func (w *textWrapper) wrapChunks(chunks []string) ([]string, object.Object) {
	lines := []string{}
	if w.width <= 0 {
		return nil, errors.NewValueError("invalid width %d (must be > 0)", w.width)
	}
	if w.maxLines > 0 {
		indent := w.initialIndent
		if w.maxLines > 1 {
			indent = w.subsequentIndent
		}
		if runesLen(indent)+runesLen(strings.TrimLeft(w.placeholder, " \t\n\r\v\f")) > w.width {
			return nil, errors.NewValueError("placeholder too large for max width")
		}
	}
	isBlank := func(s string) bool { return strings.TrimSpace(s) == "" }
	// Reverse so the next chunk is at the end (pop from the back).
	for i, j := 0, len(chunks)-1; i < j; i, j = i+1, j-1 {
		chunks[i], chunks[j] = chunks[j], chunks[i]
	}
	for len(chunks) > 0 {
		var curLine []string
		curLen := 0
		indent := w.initialIndent
		if len(lines) > 0 {
			indent = w.subsequentIndent
		}
		width := w.width - runesLen(indent)

		if w.dropWhitespace && isBlank(chunks[len(chunks)-1]) && len(lines) > 0 {
			chunks = chunks[:len(chunks)-1]
		}
		for len(chunks) > 0 {
			l := runesLen(chunks[len(chunks)-1])
			if curLen+l <= width {
				curLine = append(curLine, chunks[len(chunks)-1])
				chunks = chunks[:len(chunks)-1]
				curLen += l
			} else {
				break
			}
		}
		if len(chunks) > 0 && runesLen(chunks[len(chunks)-1]) > width {
			chunks, curLine = w.handleLongWord(chunks, curLine, curLen, width)
			curLen = 0
			for _, c := range curLine {
				curLen += runesLen(c)
			}
		}
		if w.dropWhitespace && len(curLine) > 0 && isBlank(curLine[len(curLine)-1]) {
			curLen -= runesLen(curLine[len(curLine)-1])
			curLine = curLine[:len(curLine)-1]
		}
		if len(curLine) == 0 {
			continue
		}
		if w.maxLines <= 0 || len(lines)+1 < w.maxLines ||
			((len(chunks) == 0 || (w.dropWhitespace && len(chunks) == 1 && isBlank(chunks[0]))) && curLen <= width) {
			lines = append(lines, indent+strings.Join(curLine, ""))
			continue
		}
		placed := false
		for len(curLine) > 0 {
			last := curLine[len(curLine)-1]
			if !isBlank(last) && curLen+runesLen(w.placeholder) <= width {
				curLine = append(curLine, w.placeholder)
				lines = append(lines, indent+strings.Join(curLine, ""))
				placed = true
				break
			}
			curLen -= runesLen(last)
			curLine = curLine[:len(curLine)-1]
		}
		if !placed {
			if len(lines) > 0 {
				prev := strings.TrimRight(lines[len(lines)-1], " \t\n\r\v\f")
				if runesLen(prev)+runesLen(w.placeholder) <= w.width {
					lines[len(lines)-1] = prev + w.placeholder
					break
				}
			}
			lines = append(lines, indent+strings.TrimLeft(w.placeholder, " \t\n\r\v\f"))
		}
		break
	}
	return lines, nil
}

// handleLongWord mirrors TextWrapper._handle_long_word.
func (w *textWrapper) handleLongWord(chunks, curLine []string, curLen, width int) ([]string, []string) {
	spaceLeft := 1
	if width >= 1 {
		spaceLeft = width - curLen
	}
	if w.breakLongWords && spaceLeft > 0 {
		chunk := []rune(chunks[len(chunks)-1])
		end := spaceLeft
		if w.breakOnHyphens && len(chunk) > spaceLeft {
			hyphen := -1
			for k := spaceLeft - 1; k >= 0; k-- {
				if chunk[k] == '-' {
					hyphen = k
					break
				}
			}
			if hyphen > 0 && strings.Trim(string(chunk[:hyphen]), "-") != "" {
				end = hyphen + 1
			}
		}
		curLine = append(curLine, string(chunk[:end]))
		chunks[len(chunks)-1] = string(chunk[end:])
	} else if len(curLine) == 0 {
		curLine = append(curLine, chunks[len(chunks)-1])
		chunks = chunks[:len(chunks)-1]
	}
	return chunks, curLine
}
