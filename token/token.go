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
	POW_EQ      = "**="
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
	FROM     = "FROM"
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
	DEL      = "DEL"
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
	MATCH    = "MATCH"
	CASE     = "CASE"
	WITH     = "WITH"
	AT       = "@"
)

func LookupIdent(ident string) TokenType {
	switch len(ident) {
	case 2:
		switch ident {
		case "if":
			return IF
		case "in":
			return IN
		case "is":
			return IS
		case "as":
			return AS
		case "or":
			return OR
		}
	case 3:
		switch ident {
		case "and":
			return AND
		case "for":
			return FOR
		case "def":
			return DEF
		case "del":
			return DEL
		case "not":
			return NOT
		case "try":
			return TRY
		}
	case 4:
		switch ident {
		case "True":
			return TRUE
		case "None":
			return NONE
		case "from":
			return FROM
		case "elif":
			return ELIF
		case "else":
			return ELSE
		case "pass":
			return PASS
		case "or":
			return OR
		case "with":
			return WITH
		}
	case 5:
		switch ident {
		case "False":
			return FALSE
		case "while":
			return WHILE
		case "class":
			return CLASS
		case "break":
			return BREAK
		case "raise":
			return RAISE
		case "assert":
			return ASSERT
		}
	case 6:
		switch ident {
		case "import":
			return IMPORT
		case "return":
			return RETURN
		case "lambda":
			return LAMBDA
		case "assert":
			return ASSERT
		case "except":
			return EXCEPT
		case "global":
			return GLOBAL
		}
	case 7:
		switch ident {
		case "finally":
			return FINALLY
		}
	case 8:
		switch ident {
		case "continue":
			return CONTINUE
		case "nonlocal":
			return NONLOCAL
		}
	}
	return IDENT
}
