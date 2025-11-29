package token

type TokenType string

type Token struct {
	Type    TokenType
	Literal string
	Line    int
}

const (
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"

	IDENT    = "IDENT"
	INT      = "INT"
	FLOAT    = "FLOAT"
	STRING   = "STRING"
	F_STRING = "F_STRING"

	ASSIGN      = "="
	PLUS_EQ     = "+="
	MINUS_EQ    = "-="
	MUL_EQ      = "*="
	DIV_EQ      = "/="
	FLOORDIV_EQ = "//="
	MOD_EQ      = "%="
	PLUS        = "+"
	MINUS       = "-"
	ASTERISK    = "*"
	POW         = "**"
	SLASH       = "/"
	FLOORDIV    = "//"
	PERCENT     = "%"

	// Bitwise operators
	TILDE     = "~"
	AMPERSAND = "&"
	PIPE      = "|"
	CARET     = "^"
	LSHIFT    = "<<"
	RSHIFT    = ">>"
	AND_EQ    = "&="
	OR_EQ     = "|="
	XOR_EQ    = "^="
	LSHIFT_EQ = "<<="
	RSHIFT_EQ = ">>="

	EQ     = "=="
	NOT_EQ = "!="
	LT     = "<"
	GT     = ">"
	LTE    = "<="
	GTE    = ">="

	LPAREN    = "("
	RPAREN    = ")"
	LBRACKET  = "["
	RBRACKET  = "]"
	LBRACE    = "{"
	RBRACE    = "}"
	COLON     = ":"
	COMMA     = ","
	DOT       = "."
	SEMICOLON = ";"
	NEWLINE   = "NEWLINE"
	INDENT    = "INDENT"
	DEDENT    = "DEDENT"

	TRUE     = "TRUE"
	FALSE    = "FALSE"
	NONE     = "NONE"
	IMPORT   = "IMPORT"
	IF       = "IF"
	ELIF     = "ELIF"
	ELSE     = "ELSE"
	WHILE    = "WHILE"
	FOR      = "FOR"
	IN       = "IN"
	DEF      = "DEF"
	CLASS    = "CLASS"
	RETURN   = "RETURN"
	BREAK    = "BREAK"
	CONTINUE = "CONTINUE"
	PASS     = "PASS"
	AND      = "AND"
	OR       = "OR"
	NOT      = "NOT"
	NOT_IN   = "NOT_IN"
	IS       = "IS"
	IS_NOT   = "IS_NOT"
	TRY      = "TRY"
	EXCEPT   = "EXCEPT"
	FINALLY  = "FINALLY"
	RAISE    = "RAISE"
	GLOBAL   = "GLOBAL"
	NONLOCAL = "NONLOCAL"
	LAMBDA   = "LAMBDA"
	AS       = "AS"
	ASSERT   = "ASSERT"
)

var keywords = map[string]TokenType{
	"True":     TRUE,
	"False":    FALSE,
	"None":     NONE,
	"import":   IMPORT,
	"if":       IF,
	"elif":     ELIF,
	"else":     ELSE,
	"while":    WHILE,
	"for":      FOR,
	"in":       IN,
	"def":      DEF,
	"class":    CLASS,
	"return":   RETURN,
	"break":    BREAK,
	"continue": CONTINUE,
	"pass":     PASS,
	"and":      AND,
	"or":       OR,
	"not":      NOT,
	"is":       IS,
	"try":      TRY,
	"except":   EXCEPT,
	"finally":  FINALLY,
	"raise":    RAISE,
	"global":   GLOBAL,
	"nonlocal": NONLOCAL,
	"lambda":   LAMBDA,
	"as":       AS,
	"assert":   ASSERT,
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
