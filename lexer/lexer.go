package lexer

import (
	"github.com/paularlott/scriptling/token"
)

type Lexer struct {
	input        string
	position     int
	readPosition int
	ch           byte
	line         int
	indentStack  []int
	pendingDedents int
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
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.MUL_EQ, Literal: "*=", Line: l.line}
		} else {
			tok = token.Token{Type: token.ASTERISK, Literal: string(l.ch), Line: l.line}
		}
		l.readChar()
	case '/':
		if l.peekChar() == '=' {
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
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.LTE, Literal: "<=", Line: l.line}
		} else {
			tok = token.Token{Type: token.LT, Literal: string(l.ch), Line: l.line}
		}
		l.readChar()
	case '>':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.GTE, Literal: ">=", Line: l.line}
		} else {
			tok = token.Token{Type: token.GT, Literal: string(l.ch), Line: l.line}
		}
		l.readChar()
	case '(':
		tok = token.Token{Type: token.LPAREN, Literal: string(l.ch), Line: l.line}
		l.readChar()
	case ')':
		tok = token.Token{Type: token.RPAREN, Literal: string(l.ch), Line: l.line}
		l.readChar()
	case ':':
		tok = token.Token{Type: token.COLON, Literal: string(l.ch), Line: l.line}
		l.readChar()
	case ',':
		tok = token.Token{Type: token.COMMA, Literal: string(l.ch), Line: l.line}
		l.readChar()
	case '.':
		tok = token.Token{Type: token.DOT, Literal: string(l.ch), Line: l.line}
		l.readChar()
	case '[':
		tok = token.Token{Type: token.LBRACKET, Literal: string(l.ch), Line: l.line}
		l.readChar()
	case ']':
		tok = token.Token{Type: token.RBRACKET, Literal: string(l.ch), Line: l.line}
		l.readChar()
	case '{':
		tok = token.Token{Type: token.LBRACE, Literal: string(l.ch), Line: l.line}
		l.readChar()
	case '}':
		tok = token.Token{Type: token.RBRACE, Literal: string(l.ch), Line: l.line}
		l.readChar()
	case '"', '\'':
		tok.Type = token.STRING
		tok.Literal = l.readString(l.ch)
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
	return l.input[position:l.position], isFloat
}

func (l *Lexer) readString(quote byte) string {
	l.readChar()
	position := l.position
	for l.ch != quote && l.ch != 0 {
		l.readChar()
	}
	str := l.input[position:l.position]
	l.readChar()
	return str
}

func (l *Lexer) readFString(quote byte) string {
	l.readChar() // consume quote
	position := l.position
	for l.ch != quote && l.ch != 0 {
		l.readChar()
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
