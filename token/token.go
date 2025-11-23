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
	
	IDENT  = "IDENT"
	INT    = "INT"
	FLOAT  = "FLOAT"
	STRING = "STRING"
	
	ASSIGN   = "="
	PLUS     = "+"
	MINUS    = "-"
	ASTERISK = "*"
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
	NEWLINE = "NEWLINE"
	INDENT  = "INDENT"
	DEDENT  = "DEDENT"
	
	TRUE   = "TRUE"
	FALSE  = "FALSE"
	IF     = "IF"
	ELSE   = "ELSE"
	WHILE  = "WHILE"
	FOR    = "FOR"
	IN     = "IN"
	DEF    = "DEF"
	RETURN = "RETURN"
	AND    = "AND"
	OR     = "OR"
	NOT    = "NOT"
)

var keywords = map[string]TokenType{
	"True":   TRUE,
	"False":  FALSE,
	"if":     IF,
	"else":   ELSE,
	"while":  WHILE,
	"for":    FOR,
	"in":     IN,
	"def":    DEF,
	"return": RETURN,
	"and":    AND,
	"or":     OR,
	"not":    NOT,
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
