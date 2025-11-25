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

	ASSIGN   = "="
	PLUS_EQ  = "+="
	MINUS_EQ = "-="
	MUL_EQ   = "*="
	DIV_EQ   = "/="
	MOD_EQ   = "%="
	PLUS     = "+"
	MINUS    = "-"
	ASTERISK = "*"
	POW      = "**"
	SLASH    = "/"
	PERCENT  = "%"

	EQ     = "=="
	NOT_EQ = "!="
	LT     = "<"
	GT     = ">"
	LTE    = "<="
	GTE    = ">="

	LPAREN   = "("
	RPAREN   = ")"
	LBRACKET = "["
	RBRACKET = "]"
	LBRACE   = "{"
	RBRACE   = "}"
	COLON    = ":"
	COMMA    = ","
	DOT      = "."
	NEWLINE  = "NEWLINE"
	INDENT   = "INDENT"
	DEDENT   = "DEDENT"

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
	RETURN   = "RETURN"
	BREAK    = "BREAK"
	CONTINUE = "CONTINUE"
	PASS     = "PASS"
	AND      = "AND"
	OR       = "OR"
	NOT      = "NOT"
	NOT_IN   = "NOT_IN"
	TRY      = "TRY"
	EXCEPT   = "EXCEPT"
	FINALLY  = "FINALLY"
	RAISE    = "RAISE"
	GLOBAL   = "GLOBAL"
	NONLOCAL = "NONLOCAL"
	LAMBDA   = "LAMBDA"
	AS       = "AS"
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
	"return":   RETURN,
	"break":    BREAK,
	"continue": CONTINUE,
	"pass":     PASS,
	"and":      AND,
	"or":       OR,
	"not":      NOT,
	"try":      TRY,
	"except":   EXCEPT,
	"finally":  FINALLY,
	"raise":    RAISE,
	"global":   GLOBAL,
	"nonlocal": NONLOCAL,
	"lambda":   LAMBDA,
	"as":       AS,
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
