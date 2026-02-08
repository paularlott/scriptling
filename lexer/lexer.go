package lexer

import (
	"strings"

	"github.com/paularlott/scriptling/token"
)

type Lexer struct {
	input          string
	position       int
	readPosition   int
	ch             byte
	line           int
	indentStack    []int
	pendingDedents int
	bracketDepth   int // Track depth of (), [], {}
}

func New(input string) *Lexer {
	l := &Lexer{input: input, line: 1, indentStack: []int{0}}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition++
}

func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition]
}

// peekN returns the character n positions ahead of current readPosition (1-based).
func (l *Lexer) peekN(n int) byte {
	idx := l.readPosition + n - 1
	if idx >= len(l.input) {
		return 0
	}
	return l.input[idx]
}

func (l *Lexer) NextToken() token.Token {
	if l.pendingDedents > 0 {
		l.pendingDedents--
		return token.Token{Type: token.DEDENT, Literal: "", Line: l.line}
	}

	l.skipWhitespaceExceptNewline()

	var tok token.Token
	tok.Line = l.line

	switch l.ch {
	case '\n':
		tok = token.Token{Type: token.NEWLINE, Literal: "\\n", Line: l.line}
		l.line++
		l.readChar()
		// Only process indentation when not inside brackets
		if l.bracketDepth == 0 {
			indent := l.countIndent()
			current := l.indentStack[len(l.indentStack)-1]
			if indent > current {
				l.indentStack = append(l.indentStack, indent)
				return token.Token{Type: token.INDENT, Literal: "", Line: l.line}
			} else if indent < current {
				for len(l.indentStack) > 1 && l.indentStack[len(l.indentStack)-1] > indent {
					l.indentStack = l.indentStack[:len(l.indentStack)-1]
					l.pendingDedents++
				}
				if l.pendingDedents > 0 {
					l.pendingDedents--
					return token.Token{Type: token.DEDENT, Literal: "", Line: l.line}
				}
			}
		} else {
			// Inside brackets, just skip whitespace to get to next token
			l.skipWhitespaceExceptNewline()
		}
		return tok
	case '=':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.EQ, Literal: "==", Line: l.line}
		} else {
			tok = token.Token{Type: token.ASSIGN, Literal: string(l.ch), Line: l.line}
		}
		l.readChar()
	case '+':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.PLUS_EQ, Literal: "+=", Line: l.line}
		} else {
			tok = token.Token{Type: token.PLUS, Literal: string(l.ch), Line: l.line}
		}
		l.readChar()
	case '-':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.MINUS_EQ, Literal: "-=", Line: l.line}
		} else {
			tok = token.Token{Type: token.MINUS, Literal: string(l.ch), Line: l.line}
		}
		l.readChar()
	case '*':
		if l.peekChar() == '*' {
			l.readChar()
			tok = token.Token{Type: token.POW, Literal: "**", Line: l.line}
			l.readChar()
		} else if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.MUL_EQ, Literal: "*=", Line: l.line}
			l.readChar()
		} else {
			tok = token.Token{Type: token.ASTERISK, Literal: string(l.ch), Line: l.line}
			l.readChar()
		}
	case '/':
		if l.peekChar() == '/' {
			l.readChar()
			if l.peekChar() == '=' {
				l.readChar()
				tok = token.Token{Type: token.FLOORDIV_EQ, Literal: "//=", Line: l.line}
			} else {
				tok = token.Token{Type: token.FLOORDIV, Literal: "//", Line: l.line}
			}
		} else if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.DIV_EQ, Literal: "/=", Line: l.line}
		} else {
			tok = token.Token{Type: token.SLASH, Literal: string(l.ch), Line: l.line}
		}
		l.readChar()
	case '%':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.MOD_EQ, Literal: "%=", Line: l.line}
		} else {
			tok = token.Token{Type: token.PERCENT, Literal: string(l.ch), Line: l.line}
		}
		l.readChar()
	case '!':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.NOT_EQ, Literal: "!=", Line: l.line}
			l.readChar()
		} else {
			tok = token.Token{Type: token.ILLEGAL, Literal: string(l.ch), Line: l.line}
			l.readChar()
		}
	case '<':
		if l.peekChar() == '<' {
			l.readChar()
			if l.peekChar() == '=' {
				l.readChar()
				tok = token.Token{Type: token.LSHIFT_EQ, Literal: "<<=", Line: l.line}
			} else {
				tok = token.Token{Type: token.LSHIFT, Literal: "<<", Line: l.line}
			}
		} else if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.LTE, Literal: "<=", Line: l.line}
		} else {
			tok = token.Token{Type: token.LT, Literal: string(l.ch), Line: l.line}
		}
		l.readChar()
	case '>':
		if l.peekChar() == '>' {
			l.readChar()
			if l.peekChar() == '=' {
				l.readChar()
				tok = token.Token{Type: token.RSHIFT_EQ, Literal: ">>=", Line: l.line}
			} else {
				tok = token.Token{Type: token.RSHIFT, Literal: ">>", Line: l.line}
			}
		} else if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.GTE, Literal: ">=", Line: l.line}
		} else {
			tok = token.Token{Type: token.GT, Literal: string(l.ch), Line: l.line}
		}
		l.readChar()
	case '(':
		tok = token.Token{Type: token.LPAREN, Literal: string(l.ch), Line: l.line}
		l.bracketDepth++
		l.readChar()
	case ')':
		tok = token.Token{Type: token.RPAREN, Literal: string(l.ch), Line: l.line}
		if l.bracketDepth > 0 {
			l.bracketDepth--
		}
		l.readChar()
	case ':':
		tok = token.Token{Type: token.COLON, Literal: string(l.ch), Line: l.line}
		l.readChar()
	case ',':
		tok = token.Token{Type: token.COMMA, Literal: string(l.ch), Line: l.line}
		l.readChar()
	case ';':
		tok = token.Token{Type: token.SEMICOLON, Literal: string(l.ch), Line: l.line}
		l.readChar()
	case '.':
		tok = token.Token{Type: token.DOT, Literal: string(l.ch), Line: l.line}
		l.readChar()
	case '[':
		tok = token.Token{Type: token.LBRACKET, Literal: string(l.ch), Line: l.line}
		l.bracketDepth++
		l.readChar()
	case ']':
		tok = token.Token{Type: token.RBRACKET, Literal: string(l.ch), Line: l.line}
		if l.bracketDepth > 0 {
			l.bracketDepth--
		}
		l.readChar()
	case '{':
		tok = token.Token{Type: token.LBRACE, Literal: string(l.ch), Line: l.line}
		l.bracketDepth++
		l.readChar()
	case '}':
		tok = token.Token{Type: token.RBRACE, Literal: string(l.ch), Line: l.line}
		if l.bracketDepth > 0 {
			l.bracketDepth--
		}
		l.readChar()
	case '~':
		tok = token.Token{Type: token.TILDE, Literal: string(l.ch), Line: l.line}
		l.readChar()
	case '&':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.AND_EQ, Literal: "&=", Line: l.line}
		} else {
			tok = token.Token{Type: token.AMPERSAND, Literal: string(l.ch), Line: l.line}
		}
		l.readChar()
	case '|':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.OR_EQ, Literal: "|=", Line: l.line}
		} else {
			tok = token.Token{Type: token.PIPE, Literal: string(l.ch), Line: l.line}
		}
		l.readChar()
	case '^':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.XOR_EQ, Literal: "^=", Line: l.line}
		} else {
			tok = token.Token{Type: token.CARET, Literal: string(l.ch), Line: l.line}
		}
		l.readChar()
	case '"', '\'':
		quote := l.ch
		// Triple-quote?
		if l.peekChar() == quote && l.peekN(2) == quote {
			tok.Type = token.STRING
			tok.Literal = l.readTripleString(quote)
		} else {
			tok.Type = token.STRING
			tok.Literal = l.readString(quote)
		}
	case 'f', 'F':
		if l.peekChar() == '"' || l.peekChar() == '\'' {
			l.readChar() // consume 'f'
			tok.Type = token.F_STRING
			tok.Literal = l.readFString(l.ch)
		} else {
			tok.Literal = l.readIdentifier()
			tok.Type = token.LookupIdent(tok.Literal)
			return tok
		}
	case 'r', 'R':
		// Raw string prefix: r"..." or r'...'
		if l.peekChar() == '"' || l.peekChar() == '\'' {
			quote := l.peekChar()
			// Triple-quoted raw string?
			if l.peekN(2) == quote {
				l.readChar() // consume 'r' so l.ch == quote
				tok.Type = token.STRING
				tok.Literal = l.readRawTripleString(quote)
			} else {
				l.readChar() // consume 'r'
				tok.Type = token.STRING
				tok.Literal = l.readRawString(quote)
			}
		} else {
			tok.Literal = l.readIdentifier()
			tok.Type = token.LookupIdent(tok.Literal)
			return tok
		}
	case '#':
		l.skipComment()
		return l.NextToken()
	case 0:
		tok.Literal = ""
		tok.Type = token.EOF
		for len(l.indentStack) > 1 {
			l.indentStack = l.indentStack[:len(l.indentStack)-1]
			l.pendingDedents++
		}
		if l.pendingDedents > 0 {
			l.pendingDedents--
			return token.Token{Type: token.DEDENT, Literal: "", Line: l.line}
		}
	default:
		if isLetter(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = token.LookupIdent(tok.Literal)
			// Check for 'not in' special case
			if tok.Type == token.NOT && l.ch == ' ' {
				// Peek ahead to see if next word is 'in'
				savedPos := l.position
				savedReadPos := l.readPosition
				savedCh := l.ch

				// Skip whitespace
				for l.ch == ' ' || l.ch == '\t' {
					l.readChar()
				}

				// Check if next identifier is 'in'
				if isLetter(l.ch) {
					nextIdent := l.readIdentifier()
					if nextIdent == "in" {
						// Return NOT_IN token
						return token.Token{Type: token.NOT_IN, Literal: "not in", Line: l.line}
					}
				}

				// Restore position if not 'not in'
				l.position = savedPos
				l.readPosition = savedReadPos
				l.ch = savedCh
			}
			// Check for 'is not' special case
			if tok.Type == token.IS && l.ch == ' ' {
				// Peek ahead to see if next word is 'not'
				savedPos := l.position
				savedReadPos := l.readPosition
				savedCh := l.ch

				// Skip whitespace
				for l.ch == ' ' || l.ch == '\t' {
					l.readChar()
				}

				// Check if next identifier is 'not'
				if isLetter(l.ch) {
					nextIdent := l.readIdentifier()
					if nextIdent == "not" {
						// Return IS_NOT token
						return token.Token{Type: token.IS_NOT, Literal: "is not", Line: l.line}
					}
				}

				// Restore position if not 'is not'
				l.position = savedPos
				l.readPosition = savedReadPos
				l.ch = savedCh
			}
			return tok
		} else if isDigit(l.ch) {
			num, isFloat := l.readNumber()
			if isFloat {
				tok.Type = token.FLOAT
			} else {
				tok.Type = token.INT
			}
			tok.Literal = num
			return tok
		} else {
			tok = token.Token{Type: token.ILLEGAL, Literal: string(l.ch), Line: l.line}
			l.readChar()
		}
	}
	return tok
}

func (l *Lexer) skipWhitespaceExceptNewline() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *Lexer) skipComment() {
	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}
}

func (l *Lexer) countIndent() int {
	count := 0
	for l.ch == ' ' || l.ch == '\t' {
		if l.ch == '\t' {
			count += 4
		} else {
			count++
		}
		l.readChar()
	}
	if l.ch == '\n' || l.ch == '#' {
		return l.indentStack[len(l.indentStack)-1]
	}
	return count
}

func (l *Lexer) readIdentifier() string {
	start := l.position
	for l.ch == '_' || (l.ch >= 'a' && l.ch <= 'z') || (l.ch >= 'A' && l.ch <= 'Z') || (l.ch >= '0' && l.ch <= '9') {
		l.readChar()
	}
	return l.input[start:l.position]
}

func (l *Lexer) readNumber() (string, bool) {
	position := l.position
	isFloat := false

	// Check for hex literal (0x or 0X)
	if l.ch == '0' && (l.peekChar() == 'x' || l.peekChar() == 'X') {
		l.readChar() // consume '0'
		l.readChar() // consume 'x' or 'X'
		for isHexDigit(l.ch) {
			l.readChar()
		}
		return l.input[position:l.position], false
	}

	// Check for binary literal (0b or 0B)
	if l.ch == '0' && (l.peekChar() == 'b' || l.peekChar() == 'B') {
		l.readChar() // consume '0'
		l.readChar() // consume 'b' or 'B'
		for l.ch == '0' || l.ch == '1' {
			l.readChar()
		}
		return l.input[position:l.position], false
	}

	// Check for octal literal (0o or 0O)
	if l.ch == '0' && (l.peekChar() == 'o' || l.peekChar() == 'O') {
		l.readChar() // consume '0'
		l.readChar() // consume 'o' or 'O'
		for l.ch >= '0' && l.ch <= '7' {
			l.readChar()
		}
		return l.input[position:l.position], false
	}

	for isDigit(l.ch) {
		l.readChar()
	}
	if l.ch == '.' && isDigit(l.peekChar()) {
		isFloat = true
		l.readChar()
		for isDigit(l.ch) {
			l.readChar()
		}
	}

	// Check for scientific notation (e.g., 1e10, 2.5e-3, 1E+5)
	if l.ch == 'e' || l.ch == 'E' {
		isFloat = true
		l.readChar() // consume 'e' or 'E'
		if l.ch == '+' || l.ch == '-' {
			l.readChar() // consume sign
		}
		for isDigit(l.ch) {
			l.readChar()
		}
	}

	return l.input[position:l.position], isFloat
}

func (l *Lexer) readString(quote byte) string {
	// l.ch is opening quote
	l.readChar() // move to first content char
	var result strings.Builder
	for l.ch != quote && l.ch != 0 {
		if l.ch == '\\' {
			l.readChar() // consume backslash
			switch l.ch {
			case 'n':
				result.WriteByte('\n')
			case 't':
				result.WriteByte('\t')
			case 'r':
				result.WriteByte('\r')
			case '\\':
				result.WriteByte('\\')
			case '\'':
				result.WriteByte('\'')
			case '"':
				result.WriteByte('"')
			case '0':
				result.WriteByte(0)
			default:
				// Keep backslash and the character as-is
				result.WriteByte('\\')
				result.WriteByte(l.ch)
			}
		} else {
			result.WriteByte(l.ch)
		}
		l.readChar()
	}
	l.readChar() // consume closing quote
	return result.String()
}

// readRawString reads a raw string but allows quoted characters preceded by a backslash
// to remain inside the string (so regex patterns like r'href=["\'](.*?)["\']' work).
func (l *Lexer) readRawString(quote byte) string {
	// Use index-based scanning to find closing quote that is not escaped.
	// l.ch is opening quote and l.position points to that quote index.
	opening := l.position
	start := opening + 1
	i := start
	inputLen := len(l.input)
	for i < inputLen {
		if l.input[i] == quote {
			// Count preceding backslashes
			j := i - 1
			bs := 0
			for j >= start && l.input[j] == '\\' {
				bs++
				j--
			}
			if bs%2 == 0 {
				// closing quote found
				str := l.input[start:i]
				// Advance lexer state to character after the closing quote
				nextIdx := i + 1
				if nextIdx >= inputLen {
					l.position = inputLen
					l.readPosition = inputLen
					l.ch = 0
				} else {
					l.position = nextIdx
					l.readPosition = nextIdx + 1
					l.ch = l.input[nextIdx]
				}
				return str
			}
		}
		i++
	}
	// Unterminated: return rest
	str := l.input[start:]
	l.position = inputLen
	l.readPosition = inputLen
	l.ch = 0
	return str
}

// readTripleString reads a triple-quoted string (”'...”' or """...""").
// Entry: current l.ch is the opening quote (either ' or ").
func (l *Lexer) readTripleString(quote byte) string {
	// Consume the three opening quotes
	l.readChar()
	l.readChar()
	l.readChar()
	position := l.position
	for l.ch != 0 {
		if l.ch == quote && l.peekChar() == quote && l.peekN(2) == quote {
			break
		}
		l.readChar()
	}
	str := l.input[position:l.position]
	// Consume the three closing quotes if present
	if l.ch == quote {
		l.readChar()
		if l.ch == quote {
			l.readChar()
			if l.ch == quote {
				l.readChar()
			}
		}
	}
	return str
}

// readRawTripleString reads a raw triple-quoted string; backslashes are treated literally.
func (l *Lexer) readRawTripleString(quote byte) string {
	// Consume the three opening quotes
	l.readChar()
	l.readChar()
	l.readChar()
	position := l.position
	for l.ch != 0 {
		if l.ch == quote && l.peekChar() == quote && l.peekN(2) == quote {
			break
		}
		l.readChar()
	}
	str := l.input[position:l.position]
	// Consume the three closing quotes if present
	if l.ch == quote {
		l.readChar()
		if l.ch == quote {
			l.readChar()
			if l.ch == quote {
				l.readChar()
			}
		}
	}
	return str
}

func (l *Lexer) readFString(quote byte) string {
	l.readChar() // consume quote
	position := l.position
	for l.ch != quote && l.ch != 0 {
		if l.ch == '\\' {
			l.readChar() // skip backslash
			if l.ch != 0 {
				l.readChar() // skip escaped character
			}
		} else {
			l.readChar()
		}
	}
	str := l.input[position:l.position]
	l.readChar() // consume closing quote
	return str
}

func isLetter(ch byte) bool {
	return ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isHexDigit(ch byte) bool {
	return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}
