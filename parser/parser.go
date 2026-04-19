package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/token"
)

const (
	LOWEST_PRECEDENCE = 0
	LOWEST            = 1
	CONDITIONAL       = 2 // for conditional expressions (x if cond else y)
	OR                = 3
	BIT_OR            = 4
	BIT_XOR           = 5
	BIT_AND           = 6
	AND               = 7
	EQUALS            = 8
	LESSGREATER       = 9
	BIT_SHIFT         = 10
	SUM               = 11
	PRODUCT           = 12
	POWER             = 13
	PREFIX            = 14
	CALL              = 15
)

func precedenceFor(tok token.TokenType) int {
	switch tok {
	case token.IF:
		return CONDITIONAL
	case token.OR:
		return OR
	case token.PIPE:
		return BIT_OR
	case token.CARET:
		return BIT_XOR
	case token.AMPERSAND:
		return BIT_AND
	case token.AND:
		return AND
	case token.EQ, token.NOT_EQ, token.IN, token.NOT_IN, token.IS, token.IS_NOT:
		return EQUALS
	case token.LT, token.GT, token.LTE, token.GTE:
		return LESSGREATER
	case token.LSHIFT, token.RSHIFT:
		return BIT_SHIFT
	case token.PLUS, token.MINUS:
		return SUM
	case token.SLASH, token.FLOORDIV, token.ASTERISK, token.PERCENT:
		return PRODUCT
	case token.POW:
		return POWER
	case token.LPAREN, token.LBRACKET, token.DOT:
		return CALL
	default:
		return LOWEST
	}
}

type Parser struct {
	l        *lexer.Lexer
	errors   []string
	errorBuf [8]string

	curToken       token.Token
	peekToken      token.Token
	skippedNewline bool // true if a NEWLINE was skipped between curToken and peekToken
	parenDepth     int  // track parenthesis depth for multiline support
}

type (
	prefixParseFn func(*Parser) ast.Expression
	infixParseFn  func(*Parser, ast.Expression) ast.Expression
)

func prefixParseFnFor(t token.TokenType) prefixParseFn {
	switch t {
	case token.IDENT:
		return (*Parser).parseIdentifier
	case token.INT:
		return (*Parser).parseIntegerLiteral
	case token.FLOAT:
		return (*Parser).parseFloatLiteral
	case token.STRING:
		return (*Parser).parseStringLiteral
	case token.F_STRING:
		return (*Parser).parseFStringLiteral
	case token.TRUE, token.FALSE:
		return (*Parser).parseBoolean
	case token.NONE:
		return (*Parser).parseNone
	case token.MINUS, token.NOT, token.TILDE:
		return (*Parser).parsePrefixExpression
	case token.LPAREN:
		return (*Parser).parseGroupedExpression
	case token.LBRACKET:
		return (*Parser).parseListLiteral
	case token.LBRACE:
		return (*Parser).parseDictLiteral
	case token.LAMBDA:
		return (*Parser).parseLambda
	default:
		return nil
	}
}

func infixParseFnFor(t token.TokenType) infixParseFn {
	switch t {
	case token.PLUS, token.MINUS, token.SLASH, token.FLOORDIV, token.ASTERISK,
		token.POW, token.PERCENT, token.EQ, token.NOT_EQ, token.LT, token.GT,
		token.LTE, token.GTE, token.AND, token.OR, token.IN, token.NOT_IN,
		token.IS, token.IS_NOT, token.AMPERSAND, token.PIPE, token.CARET,
		token.LSHIFT, token.RSHIFT:
		return (*Parser).parseInfixExpression
	case token.LPAREN:
		return (*Parser).parseCallExpression
	case token.LBRACKET, token.DOT:
		return (*Parser).parseIndexExpression
	case token.IF:
		return (*Parser).parseConditionalExpression
	default:
		return nil
	}
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{}
	p.Reset(l)
	return p
}

func (p *Parser) Reset(l *lexer.Lexer) {
	p.l = l
	p.errors = p.errorBuf[:0]
	p.curToken = token.Token{}
	p.peekToken = token.Token{}
	p.skippedNewline = false
	p.parenDepth = 0
	p.nextToken()
	p.nextToken()
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) peekError(t token.TokenType) {
	msg := fmt.Sprintf("line %d: expected next token to be %s, got %s instead",
		p.peekToken.Line, t, p.peekToken.Type)
	p.errors = append(p.errors, msg)
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
	p.skippedNewline = false

	// Track parenthesis depth based on the token we just consumed (curToken)
	if p.curToken.Type == token.LPAREN || p.curToken.Type == token.LBRACKET || p.curToken.Type == token.LBRACE {
		p.parenDepth++
	} else if p.curToken.Type == token.RPAREN || p.curToken.Type == token.RBRACKET || p.curToken.Type == token.RBRACE {
		if p.parenDepth > 0 {
			p.parenDepth--
		}
	}

	// Skip NEWLINE and SEMICOLON tokens (always skip these at top level too)
	for p.peekToken.Type == token.NEWLINE || p.peekToken.Type == token.SEMICOLON {
		p.skippedNewline = true
		p.peekToken = p.l.NextToken()
	}

	// When inside parentheses, also skip INDENT and DEDENT tokens
	// This allows multiline function calls, list literals, dict literals, etc.
	// Note: We need to check this after updating parenDepth so that when we just
	// entered a bracket, we immediately skip any INDENT/DEDENT in peekToken
	if p.parenDepth > 0 {
		for p.peekToken.Type == token.INDENT || p.peekToken.Type == token.DEDENT || p.peekToken.Type == token.NEWLINE || p.peekToken.Type == token.SEMICOLON {
			if p.peekToken.Type == token.NEWLINE || p.peekToken.Type == token.SEMICOLON {
				p.skippedNewline = true
			}
			p.peekToken = p.l.NextToken()
		}
	}
}

func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

func (p *Parser) peekPrecedence() int {
	return precedenceFor(p.peekToken.Type)
}

func (p *Parser) curPrecedence() int {
	return precedenceFor(p.curToken.Type)
}

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{Statements: make([]ast.Statement, 0, 4)}

	for !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.NEWLINE) || p.curTokenIs(token.SEMICOLON) || p.curTokenIs(token.INDENT) || p.curTokenIs(token.DEDENT) {
			p.nextToken()
			continue
		}
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}

	return program
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.IMPORT:
		return p.parseImportStatement()
	case token.FROM:
		return p.parseFromImportStatement()
	case token.DEF:
		return p.parseFunctionStatement()
	case token.CLASS:
		return p.parseClassStatement()
	case token.IF:
		return p.parseIfStatement()
	case token.WHILE:
		return p.parseWhileStatement()
	case token.FOR:
		return p.parseForStatement()
	case token.RETURN:
		return p.parseReturnStatement()
	case token.BREAK:
		return &ast.BreakStatement{Token: p.curToken}
	case token.CONTINUE:
		return &ast.ContinueStatement{Token: p.curToken}
	case token.PASS:
		return &ast.PassStatement{Token: p.curToken}
	case token.DEL:
		return p.parseDelStatement()
	case token.TRY:
		return p.parseTryStatement()
	case token.RAISE:
		return p.parseRaiseStatement()
	case token.GLOBAL:
		return p.parseGlobalStatement()
	case token.NONLOCAL:
		return p.parseNonlocalStatement()
	case token.ASSERT:
		return p.parseAssertStatement()
	case token.WITH:
		return p.parseWithStatement()
	case token.AT:
		return p.parseDecoratedStatement()
	case token.IDENT:
		if p.curToken.Literal == "match" && !p.peekTokenIs(token.ASSIGN) && !p.peekTokenIs(token.COMMA) && !p.isAugmentedAssign() && !p.peekTokenIs(token.LPAREN) && !p.peekTokenIs(token.DOT) && !p.peekTokenIs(token.LBRACKET) {
			return p.parseMatchStatement()
		}
		if p.peekTokenIs(token.ASSIGN) {
			return p.parseAssignStatement()
		} else if p.peekTokenIs(token.COMMA) {
			return p.parseMultipleAssignStatement()
		} else if p.isAugmentedAssign() {
			return p.parseAugmentedAssignStatement()
		}
		return p.parseExpressionStatement()
	case token.ASTERISK:
		// Check if this is starred unpacking: *a, b = ...
		if p.peekTokenIs(token.IDENT) {
			return p.parseMultipleAssignStatement()
		}
		return p.parseExpressionStatement()
	default:
		return p.parseExpressionStatement()
	}
}

func (p *Parser) isAugmentedAssign() bool {
	return p.peekTokenIs(token.PLUS_EQ) || p.peekTokenIs(token.MINUS_EQ) ||
		p.peekTokenIs(token.MUL_EQ) || p.peekTokenIs(token.DIV_EQ) || p.peekTokenIs(token.FLOORDIV_EQ) ||
		p.peekTokenIs(token.MOD_EQ) || p.peekTokenIs(token.POW_EQ) ||
		p.peekTokenIs(token.AND_EQ) || p.peekTokenIs(token.OR_EQ) || p.peekTokenIs(token.XOR_EQ) ||
		p.peekTokenIs(token.LSHIFT_EQ) || p.peekTokenIs(token.RSHIFT_EQ)
}

func (p *Parser) parseAssignStatement() *ast.AssignStatement {
	stmt := &ast.AssignStatement{Token: p.curToken}
	stmt.Left = p.parseExpression(LOWEST)

	if !p.expectPeek(token.ASSIGN) {
		return nil
	}

	p.nextToken()

	// Handle chained assignment: a = b = 5
	// Peek ahead: if we have IDENT = ... then parse the inner assignment first
	if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.ASSIGN) {
		inner := p.parseAssignStatement()
		if inner == nil {
			return nil
		}
		// The value of the outer assignment is the same as the inner's value
		stmt.Value = inner.Value
		// Wrap as a block: evaluate inner first, then assign same value to outer
		// We do this by making the value a ChainedAssign expression
		// Simplest approach: store inner as a preceding statement via a sequence
		// Actually: just assign inner.Value to both. Return a synthetic block.
		// For simplicity, return the inner statement and let the outer be a separate assign.
		// We need both to execute, so use the existing AST by returning a sequence.
		// The cleanest approach without new AST nodes: evaluate inner, use its value.
		stmt.Value = inner.Value
		// We need inner to also execute. Embed it as a ChainedAssign.
		// Since we don't have a sequence node, we'll add a Chained field to AssignStatement.
		stmt.Chained = inner
		return stmt
	}

	first := p.parseExpressionWithConditional()
	stmt.Value = p.parseTuplePackingTail(stmt.Token, first)

	return stmt
}

func (p *Parser) parseMultipleAssignStatement() ast.Statement {
	names := make([]*ast.Identifier, 0, 4)
	starredIndex := -1

	// Parse first identifier (may be starred)
	if p.curTokenIs(token.ASTERISK) {
		starredIndex = 0
		if !p.expectPeek(token.IDENT) {
			return nil
		}
	}
	names = append(names, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})

	// Parse remaining identifiers
	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // consume comma
		p.nextToken() // move to next token

		// Check for starred identifier
		if p.curTokenIs(token.ASTERISK) {
			if starredIndex != -1 {
				p.errors = append(p.errors, "multiple starred expressions in assignment")
				return nil
			}
			starredIndex = len(names)
			if !p.expectPeek(token.IDENT) {
				return nil
			}
		} else if !p.curTokenIs(token.IDENT) {
			p.errors = append(p.errors, fmt.Sprintf("expected identifier, got %s", p.curToken.Type))
			return nil
		}
		names = append(names, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})
	}

	if !p.expectPeek(token.ASSIGN) {
		return nil
	}

	p.nextToken()

	// Parse the value - check if it's a comma-separated list (tuple packing)
	firstValue := p.parseExpression(LOWEST)

	// If there's a comma after the first value, it's tuple packing
	if p.peekTokenIs(token.COMMA) {
		values := make([]ast.Expression, 1, 4)
		values[0] = firstValue
		for p.peekTokenIs(token.COMMA) {
			p.nextToken() // consume comma
			p.nextToken() // move to next expression
			values = append(values, p.parseExpression(LOWEST))
		}
		// Create a tuple literal from the values
		value := &ast.TupleLiteral{
			Token:    names[0].Token,
			Elements: values,
		}
		return &ast.MultipleAssignStatement{
			Token:        names[0].Token,
			Names:        names,
			Value:        value,
			StarredIndex: starredIndex,
		}
	}

	// Single value (must be a tuple/list to unpack)
	return &ast.MultipleAssignStatement{
		Token:        names[0].Token,
		Names:        names,
		Value:        firstValue,
		StarredIndex: starredIndex,
	}
}

func (p *Parser) parseAugmentedAssignStatement() *ast.AugmentedAssignStatement {
	stmt := &ast.AugmentedAssignStatement{Token: p.curToken}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	p.nextToken()
	stmt.Operator = p.curToken.Literal

	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)

	return stmt
}

func (p *Parser) parseImportStatement() *ast.ImportStatement {
	stmt := &ast.ImportStatement{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	name, tok, ok := p.parseDottedName()
	if !ok {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: tok, Value: name}

	// Check for alias (as)
	if p.peekTokenIs(token.AS) {
		p.nextToken() // consume 'as'
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		stmt.Alias = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}

	// Check for additional imports separated by commas
	for p.peekTokenIs(token.COMMA) {
		if stmt.AdditionalNames == nil {
			stmt.AdditionalNames = make([]*ast.Identifier, 0, 2)
			stmt.AdditionalAliases = make([]*ast.Identifier, 0, 2)
		}
		p.nextToken() // consume comma
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		addName, tok, ok := p.parseDottedName()
		if !ok {
			return nil
		}
		stmt.AdditionalNames = append(stmt.AdditionalNames, &ast.Identifier{
			Token: tok,
			Value: addName,
		})

		// Check for alias on this additional import
		var addAlias *ast.Identifier
		if p.peekTokenIs(token.AS) {
			p.nextToken() // consume 'as'
			if !p.expectPeek(token.IDENT) {
				return nil
			}
			addAlias = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		}
		stmt.AdditionalAliases = append(stmt.AdditionalAliases, addAlias)
	}

	return stmt
}

// parseFromImportStatement parses "from X import Y, Z" or "from X import Y as A"
// Also handles relative imports: "from . import X", "from .. import X", "from .module import X"
func (p *Parser) parseFromImportStatement() *ast.FromImportStatement {
	stmt := &ast.FromImportStatement{Token: p.curToken}

	// Check for relative imports (leading dots)
	relativeLevel := 0
	for p.peekTokenIs(token.DOT) {
		p.nextToken() // consume dot
		relativeLevel++
	}

	stmt.RelativeLevel = relativeLevel

	// After dots, check what comes next:
	// - If we have dots and next is IMPORT: "from . import X" or "from .. import X"
	// - If we have dots and next is IDENT: "from .module import X"
	// - If no dots and next is IDENT: "from module import X" (absolute import)
	if relativeLevel > 0 {
		if p.peekTokenIs(token.IMPORT) {
			// Pure relative import: "from . import X" - no module name
			stmt.Module = nil
		} else if p.peekTokenIs(token.IDENT) {
			// Relative import with module: "from .module import X"
			p.nextToken() // move to identifier
			moduleName, tok, ok := p.parseDottedName()
			if !ok {
				return nil
			}
			stmt.Module = &ast.Identifier{Token: tok, Value: moduleName}
		} else {
			p.errors = append(p.errors, fmt.Sprintf("line %d: expected identifier or 'import' after relative import dots", p.curToken.Line))
			return nil
		}
	} else {
		// Absolute import: "from module import X"
		if !p.peekTokenIs(token.IDENT) {
			p.errors = append(p.errors, fmt.Sprintf("line %d: expected module name after 'from'", p.curToken.Line))
			return nil
		}
		p.nextToken() // move to identifier

		moduleName, tok, ok := p.parseDottedName()
		if !ok {
			return nil
		}
		stmt.Module = &ast.Identifier{Token: tok, Value: moduleName}
	}

	// Expect 'import' keyword
	if !p.expectPeek(token.IMPORT) {
		return nil
	}

	// Parse the names to import
	if !p.expectPeek(token.IDENT) {
		return nil
	}

	// First name
	name := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	stmt.Names = make([]*ast.Identifier, 0, 2)
	stmt.Aliases = make([]*ast.Identifier, 0, 2)
	stmt.Names = append(stmt.Names, name)

	// Check for alias (as)
	if p.peekTokenIs(token.AS) {
		p.nextToken() // consume 'as'
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		stmt.Aliases = append(stmt.Aliases, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})
	} else {
		stmt.Aliases = append(stmt.Aliases, nil)
	}

	// Parse additional names separated by commas
	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // consume comma
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		name := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		stmt.Names = append(stmt.Names, name)

		// Check for alias
		if p.peekTokenIs(token.AS) {
			p.nextToken() // consume 'as'
			if !p.expectPeek(token.IDENT) {
				return nil
			}
			stmt.Aliases = append(stmt.Aliases, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})
		} else {
			stmt.Aliases = append(stmt.Aliases, nil)
		}
	}

	return stmt
}

func (p *Parser) parseDottedName() (string, token.Token, bool) {
	var builder strings.Builder
	tok := p.curToken
	builder.WriteString(p.curToken.Literal)
	for p.peekTokenIs(token.DOT) {
		p.nextToken() // consume dot
		if !p.expectPeek(token.IDENT) {
			return "", token.Token{}, false
		}
		builder.WriteByte('.')
		builder.WriteString(p.curToken.Literal)
		tok = p.curToken
	}
	return builder.String(), tok, true
}

func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	stmt := &ast.ReturnStatement{Token: p.curToken}

	if !p.peekTokenIs(token.NEWLINE) && !p.peekTokenIs(token.SEMICOLON) && !p.peekTokenIs(token.EOF) && !p.peekTokenIs(token.DEDENT) {
		p.nextToken()
		first := p.parseExpressionWithConditional()
		stmt.ReturnValue = p.parseTuplePackingTail(stmt.Token, first)
	}

	return stmt
}

func (p *Parser) parseDelStatement() *ast.DelStatement {
	stmt := &ast.DelStatement{Token: p.curToken}

	if p.peekTokenIs(token.NEWLINE) || p.peekTokenIs(token.SEMICOLON) || p.peekTokenIs(token.EOF) || p.peekTokenIs(token.DEDENT) {
		p.errors = append(p.errors, fmt.Sprintf("line %d: expected target after 'del'", p.curToken.Line))
		return nil
	}

	p.nextToken()
	stmt.Target = p.parseExpression(LOWEST)

	return stmt
}

func (p *Parser) parseExpressionStatement() ast.Statement {
	expr := p.parseExpressionWithConditional()
	if p.peekTokenIs(token.ASSIGN) {
		stmt := &ast.AssignStatement{Token: p.curToken, Left: expr}
		p.nextToken() // consume =
		p.nextToken() // move to value
		first := p.parseExpressionWithConditional()
		stmt.Value = p.parseTuplePackingTail(p.curToken, first)
		return stmt
	}
	expr = p.parseTuplePackingTail(p.curToken, expr)
	return &ast.ExpressionStatement{Token: p.curToken, Expression: expr}
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	prefix := prefixParseFnFor(p.curToken.Type)
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	leftExp := prefix(p)

	for {
		peekType := p.peekToken.Type
		switch peekType {
		case token.NEWLINE, token.SEMICOLON, token.EOF, token.COLON:
			return leftExp
		}
		if precedence >= precedenceFor(peekType) {
			return leftExp
		}

		// Special handling for IF token: only treat as conditional expression
		// if it appears on the same line (no newline was skipped)
		if peekType == token.IF && p.skippedNewline {
			return leftExp
		}

		// Don't continue parsing infix expressions across newlines at top level
		// This prevents "a = 1\n*b, c = [2, 3]" from being parsed as "a = 1 * b"
		if p.parenDepth == 0 && p.skippedNewline {
			return leftExp
		}

		infix := infixParseFnFor(peekType)
		if infix == nil {
			return leftExp
		}
		p.nextToken()
		leftExp = infix(p, leftExp)
	}
}

// parseConditionalExpression parses conditional expressions (x if cond else y)
// Called as an infix parser when IF is seen after an expression
func (p *Parser) parseConditionalExpression(trueExpr ast.Expression) ast.Expression {
	ifToken := p.curToken
	p.nextToken() // move to condition
	condition := p.parseExpression(LOWEST)

	if !p.expectPeek(token.ELSE) {
		return nil
	}
	p.nextToken() // move to false expression
	// Parse false expression with CONDITIONAL precedence to handle nested conditionals
	falseExpr := p.parseExpression(CONDITIONAL)
	return &ast.ConditionalExpression{
		Token:     ifToken,
		TrueExpr:  trueExpr,
		Condition: condition,
		FalseExpr: falseExpr,
	}
}

// parseExpressionWithConditional parses expressions including conditional expressions (x if cond else y)
// This is now just a wrapper that calls parseExpression with LOWEST precedence
// The actual conditional expression handling is done via the registered infix parser
func (p *Parser) parseExpressionWithConditional() ast.Expression {
	return p.parseExpression(LOWEST)
}

// parseTuplePackingTail checks for a trailing comma indicating implicit tuple packing.
// tok is the token to use for the TupleLiteral node.
func (p *Parser) parseTuplePackingTail(tok token.Token, first ast.Expression) ast.Expression {
	if !p.peekTokenIs(token.COMMA) {
		return first
	}
	elements := make([]ast.Expression, 1, 4)
	elements[0] = first
	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // consume comma — skippedNewline is now set if a newline followed
		if p.skippedNewline || p.peekTokenIs(token.EOF) || p.peekTokenIs(token.DEDENT) {
			break // trailing comma at end of line
		}
		p.nextToken()
		elements = append(elements, p.parseExpressionWithConditional())
	}
	return &ast.TupleLiteral{Token: tok, Elements: elements}
}

func (p *Parser) noPrefixParseFnError(t token.TokenType) {
	msg := fmt.Sprintf("line %d: no prefix parse function for %s found", p.curToken.Line, t)
	p.errors = append(p.errors, msg)
}

func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	lit := &ast.IntegerLiteral{Token: p.curToken}
	if value, ok := parseFastIntegerLiteral(p.curToken.Literal); ok {
		lit.Value = value
		return lit
	}
	value, err := strconv.ParseInt(p.curToken.Literal, 0, 64)
	if err != nil {
		msg := fmt.Sprintf("line %d: could not parse %q as integer", p.curToken.Line, p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}
	lit.Value = value
	return lit
}

func parseFastIntegerLiteral(s string) (int64, bool) {
	if len(s) == 0 || len(s) > 19 {
		return 0, false
	}
	var value int64
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch < '0' || ch > '9' {
			return 0, false
		}
		digit := int64(ch - '0')
		if value > (1<<63-1-digit)/10 {
			return 0, false
		}
		value = value*10 + digit
	}
	return value, true
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	lit := &ast.FloatLiteral{Token: p.curToken}
	value, err := strconv.ParseFloat(p.curToken.Literal, 64)
	if err != nil {
		msg := fmt.Sprintf("line %d: could not parse %q as float", p.curToken.Line, p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}
	lit.Value = value
	return lit
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return p.parseAdjacentStrings(&ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal})
}

func (p *Parser) parseFStringLiteral() ast.Expression {
	fstr := &ast.FStringLiteral{Token: p.curToken}
	fstr.Parts, fstr.Expressions, fstr.FormatSpecs = p.parseFStringContent(p.curToken.Literal)
	return p.parseAdjacentStrings(fstr)
}

// parseAdjacentStrings handles implicit string concatenation (Python-style).
// Adjacent string/f-string literals are concatenated: "hello" " world" → "hello world"
// Plain string + plain string is merged at parse time (zero runtime cost).
// Mixed string + f-string creates an InfixExpression with "+" operator.
// Only concatenates on the same logical line, or across lines inside parens/brackets.
func (p *Parser) parseAdjacentStrings(left ast.Expression) ast.Expression {
	if leftStr, ok := left.(*ast.StringLiteral); ok {
		var builder strings.Builder
		usedBuilder := false
		for (p.parenDepth > 0 || !p.skippedNewline) && p.peekTokenIs(token.STRING) {
			if !usedBuilder {
				builder.Grow(len(leftStr.Value) + len(p.peekToken.Literal) + 8)
				builder.WriteString(leftStr.Value)
				usedBuilder = true
			}
			p.nextToken()
			builder.WriteString(p.curToken.Literal)
		}
		if usedBuilder {
			leftStr.Value = builder.String()
		}
	}

	for (p.parenDepth > 0 || !p.skippedNewline) && (p.peekTokenIs(token.STRING) || p.peekTokenIs(token.F_STRING)) {
		tok := p.curToken
		p.nextToken()

		var right ast.Expression
		if p.curTokenIs(token.STRING) {
			right = &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
		} else {
			fstr := &ast.FStringLiteral{Token: p.curToken}
			fstr.Parts, fstr.Expressions, fstr.FormatSpecs = p.parseFStringContent(p.curToken.Literal)
			right = fstr
		}

		left = &ast.InfixExpression{
			Token:    tok,
			Operator: "+",
			Left:     left,
			Right:    right,
		}
	}
	return left
}

func (p *Parser) parseFStringContent(content string) ([]string, []ast.Expression, []string) {
	parts := make([]string, 0, 4)
	expressions := make([]ast.Expression, 0, 2)
	formatSpecs := make([]string, 0, 2)
	var current strings.Builder
	i := 0

	for i < len(content) {
		if content[i] == '{' && i+1 < len(content) && content[i+1] != '{' {
			// Found expression start
			parts = append(parts, current.String())
			current.Reset()
			i++ // skip {

			// Extract expression until : or }
			var exprStr strings.Builder
			var formatSpec strings.Builder
			for i < len(content) && content[i] != '}' && content[i] != ':' {
				exprStr.WriteByte(content[i])
				i++
			}

			// Check for format specifier
			if i < len(content) && content[i] == ':' {
				i++ // skip :
				// Extract format spec until }
				for i < len(content) && content[i] != '}' {
					formatSpec.WriteByte(content[i])
					i++
				}
			}

			if i < len(content) {
				i++ // skip }
			}

			// Parse the expression
			exprText := exprStr.String()
			if exprText != "" {
				expr := parseExpressionString(exprText)
				if expr != nil {
					expressions = append(expressions, expr)
					formatSpecs = append(formatSpecs, formatSpec.String())
				}
			}
		} else if content[i] == '{' && i+1 < len(content) && content[i+1] == '{' {
			// Escaped brace
			current.WriteByte('{')
			i += 2
		} else if content[i] == '}' && i+1 < len(content) && content[i+1] == '}' {
			// Escaped brace
			current.WriteByte('}')
			i += 2
		} else if content[i] == '\\' && i+1 < len(content) {
			// Handle escape sequences
			i++ // consume backslash
			switch content[i] {
			case 'n':
				current.WriteByte('\n')
			case 't':
				current.WriteByte('\t')
			case 'r':
				current.WriteByte('\r')
			case '\\':
				current.WriteByte('\\')
			case '\'':
				current.WriteByte('\'')
			case '"':
				current.WriteByte('"')
			case '0':
				current.WriteByte(0)
			default:
				// Keep backslash and the character as-is
				current.WriteByte('\\')
				current.WriteByte(content[i])
			}
			i++
		} else {
			current.WriteByte(content[i])
			i++
		}
	}

	parts = append(parts, current.String())
	return parts, expressions, formatSpecs
}

func parseExpressionString(input string) ast.Expression {
	p := New(lexer.New(input))
	expr := p.parseExpression(LOWEST)
	return expr
}

func (p *Parser) parseBoolean() ast.Expression {
	return &ast.Boolean{Token: p.curToken, Value: p.curTokenIs(token.TRUE)}
}

func (p *Parser) parseNone() ast.Expression {
	return &ast.None{Token: p.curToken}
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	expression := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}
	p.nextToken()
	expression.Right = p.parseExpression(PREFIX)
	return expression
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expression := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}
	precedence := p.curPrecedence()
	currentOp := p.curToken.Literal
	p.nextToken()
	expression.Right = p.parseExpression(precedence)

	// Check for chained comparisons: a < b < c becomes a < b and b < c
	if isComparisonOp(currentOp) && (p.peekTokenIs(token.LT) || p.peekTokenIs(token.GT) ||
		p.peekTokenIs(token.LTE) || p.peekTokenIs(token.GTE) ||
		p.peekTokenIs(token.EQ) || p.peekTokenIs(token.NOT_EQ)) {
		// Build chained comparison
		comparisons := []*ast.InfixExpression{expression}

		for isComparisonOp(currentOp) && (p.peekTokenIs(token.LT) || p.peekTokenIs(token.GT) ||
			p.peekTokenIs(token.LTE) || p.peekTokenIs(token.GTE) ||
			p.peekTokenIs(token.EQ) || p.peekTokenIs(token.NOT_EQ)) {
			p.nextToken() // consume comparison operator
			nextOp := p.curToken.Literal
			nextComp := &ast.InfixExpression{
				Token:    p.curToken,
				Operator: nextOp,
				Left:     expression.Right, // Use previous right as new left
			}
			p.nextToken()
			nextComp.Right = p.parseExpression(precedence)
			comparisons = append(comparisons, nextComp)
			expression = nextComp
			currentOp = nextOp
		}

		// Build the and chain: comp1 and comp2 and comp3...
		if len(comparisons) > 1 {
			result := ast.Expression(comparisons[0])
			for i := 1; i < len(comparisons); i++ {
				result = &ast.InfixExpression{
					Token:    comparisons[0].Token, // Use first comparison token
					Operator: "and",
					Left:     result,
					Right:    comparisons[i],
				}
			}
			return result
		}
	}

	return expression
}

func isComparisonOp(op string) bool {
	return op == "<" || op == ">" || op == "<=" || op == ">=" || op == "==" || op == "!="
}

func (p *Parser) parseGroupedExpression() ast.Expression {
	p.nextToken()

	// Check for empty tuple
	if p.curTokenIs(token.RPAREN) {
		return &ast.TupleLiteral{Token: p.curToken, Elements: nil}
	}

	firstExp := p.parseExpression(LOWEST_PRECEDENCE)

	// Check if this is a generator expression (similar to list comprehension)
	if p.peekTokenIs(token.FOR) {
		return p.parseGeneratorExpression(firstExp)
	}

	// Check if this is a tuple (has comma)
	if p.peekTokenIs(token.COMMA) {
		elements := make([]ast.Expression, 1, 4)
		elements[0] = firstExp

		for p.peekTokenIs(token.COMMA) {
			p.nextToken() // consume comma
			if p.peekTokenIs(token.RPAREN) {
				// Trailing comma case
				break
			}
			p.nextToken()
			elements = append(elements, p.parseExpression(LOWEST_PRECEDENCE))
		}

		if !p.expectPeek(token.RPAREN) {
			return nil
		}

		return &ast.TupleLiteral{Token: p.curToken, Elements: elements}
	}

	// Regular grouped expression
	if !p.expectPeek(token.RPAREN) {
		return nil
	}
	return firstExp
}

func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	exp := &ast.CallExpression{Token: p.curToken, Function: function}
	exp.Arguments, exp.Keywords, exp.ArgsUnpack, exp.KwargsUnpack = p.parseCallArguments()
	return exp
}

func (p *Parser) parseCallArguments() ([]ast.Expression, map[string]ast.Expression, []ast.Expression, ast.Expression) {
	var args []ast.Expression
	var keywords map[string]ast.Expression
	var argsUnpack []ast.Expression
	var kwargsUnpack ast.Expression

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return args, keywords, nil, nil
	}

	p.nextToken()

	for {
		// Check for **kwargs unpacking
		if p.curTokenIs(token.POW) {
			p.nextToken() // move to expression
			kwargsUnpack = p.parseExpression(LOWEST)
			if p.peekTokenIs(token.COMMA) {
				p.nextToken() // consume comma
				if p.peekTokenIs(token.RPAREN) {
					break
				}
				p.nextToken()
				continue
			}
			break
		}

		// Check for *args unpacking
		if p.curTokenIs(token.ASTERISK) {
			if len(keywords) > 0 {
				msg := fmt.Sprintf("line %d: positional argument follows keyword argument", p.curToken.Line)
				p.errors = append(p.errors, msg)
				return nil, nil, nil, nil
			}
			p.nextToken() // move to expression
			argsUnpack = append(argsUnpack, p.parseExpression(LOWEST))
			if p.peekTokenIs(token.COMMA) {
				p.nextToken() // consume comma
				if p.peekTokenIs(token.RPAREN) {
					break
				}
				p.nextToken()
				continue
			}
			break
		}

		// Check for keyword argument: name=value
		if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.ASSIGN) {
			key := p.curToken.Literal
			if keywords == nil {
				keywords = make(map[string]ast.Expression, 2)
			}
			if _, exists := keywords[key]; exists {
				msg := fmt.Sprintf("line %d: keyword argument repeated: %s", p.curToken.Line, key)
				p.errors = append(p.errors, msg)
				return nil, nil, nil, nil
			}
			p.nextToken() // consume name
			p.nextToken() // consume =
			value := p.parseExpression(LOWEST)
			keywords[key] = value
		} else {
			// Positional argument
			if len(keywords) > 0 {
				msg := fmt.Sprintf("line %d: positional argument follows keyword argument", p.curToken.Line)
				p.errors = append(p.errors, msg)
				return nil, nil, nil, nil
			}

			expr := p.parseExpression(LOWEST)

			// Check for generator expression (for function arguments without parens)
			if p.peekTokenIs(token.FOR) {
				// This is a generator expression like: func(x for x in list)
				// parseGeneratorExpressionInCall expects to consume the end token (RPAREN)
				genExpr := p.parseGeneratorExpressionInCall(expr, token.RPAREN)
				if genExpr != nil {
					args = append(args, genExpr)
					return args, keywords, argsUnpack, kwargsUnpack
				}
			}

			args = append(args, expr)
		}

		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
			if p.peekTokenIs(token.RPAREN) {
				break
			}
			p.nextToken()
			continue
		}
		break
	}

	if !p.expectPeek(token.RPAREN) {
		return nil, nil, nil, nil
	}

	return args, keywords, argsUnpack, kwargsUnpack
}

func (p *Parser) parseIfStatement() *ast.IfStatement {
	stmt := &ast.IfStatement{Token: p.curToken}

	p.nextToken()
	stmt.Condition = p.parseExpression(LOWEST_PRECEDENCE)

	if !p.expectPeek(token.COLON) {
		return nil
	}

	stmt.Consequence = p.parseBlockStatement()

	// Parse elif clauses
	for p.peekTokenIs(token.ELIF) {
		if stmt.ElifClauses == nil {
			stmt.ElifClauses = make([]*ast.ElifClause, 0, 2)
		}
		p.nextToken() // consume elif
		elifClause := &ast.ElifClause{Token: p.curToken}

		p.nextToken()
		elifClause.Condition = p.parseExpression(LOWEST_PRECEDENCE)

		if !p.expectPeek(token.COLON) {
			return nil
		}

		elifClause.Consequence = p.parseBlockStatement()
		stmt.ElifClauses = append(stmt.ElifClauses, elifClause)
	}

	// Parse else clause
	if p.peekTokenIs(token.ELSE) {
		p.nextToken()
		if !p.expectPeek(token.COLON) {
			return nil
		}
		stmt.Alternative = p.parseBlockStatement()
	}

	return stmt
}

func (p *Parser) parseWhileStatement() *ast.WhileStatement {
	stmt := &ast.WhileStatement{Token: p.curToken}

	p.nextToken()
	stmt.Condition = p.parseExpression(LOWEST_PRECEDENCE)

	if !p.expectPeek(token.COLON) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()

	if p.peekTokenIs(token.ELSE) {
		p.nextToken()
		if !p.expectPeek(token.COLON) {
			return nil
		}
		stmt.Else = p.parseBlockStatement()
	}

	return stmt
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken, Statements: make([]ast.Statement, 0, 2)}

	// Check for single-line block (statement on same line after colon)
	// e.g., "if True: x = 1" or "if True: return x"
	if !p.peekTokenIs(token.NEWLINE) && !p.peekTokenIs(token.INDENT) && !p.peekTokenIs(token.EOF) {
		p.nextToken()
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		return block
	}

	if !p.expectPeek(token.INDENT) {
		return nil
	}

	p.nextToken()

	for !p.curTokenIs(token.DEDENT) && !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.NEWLINE) || p.curTokenIs(token.SEMICOLON) || p.curTokenIs(token.INDENT) {
			p.nextToken()
			continue
		}
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	return block
}

func (p *Parser) parseFunctionStatement() *ast.FunctionStatement {
	stmt := &ast.FunctionStatement{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	stmt.Function = &ast.FunctionLiteral{Token: stmt.Token}
	stmt.Function.Parameters, stmt.Function.DefaultValues, stmt.Function.Variadic, stmt.Function.Kwargs = p.parseFunctionParameters()

	if !p.expectPeek(token.COLON) {
		return nil
	}

	stmt.Function.Body = p.parseBlockStatement()

	return stmt
}

func (p *Parser) parseClassStatement() *ast.ClassStatement {
	stmt := &ast.ClassStatement{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// Check for optional base class: class Name(BaseClass):
	// BaseClass can be a dotted name like html.parser.HTMLParser
	if p.peekTokenIs(token.LPAREN) {
		p.nextToken() // consume (
		p.nextToken() // move to base class expression
		stmt.BaseClass = p.parseExpression(LOWEST)
		if !p.expectPeek(token.RPAREN) {
			return nil
		}
	}

	if !p.expectPeek(token.COLON) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()

	return stmt
}

func (p *Parser) parseFunctionParameters() ([]*ast.Identifier, map[string]ast.Expression, *ast.Identifier, *ast.Identifier) {
	var identifiers []*ast.Identifier
	var defaults map[string]ast.Expression
	var variadic *ast.Identifier
	var kwargs *ast.Identifier

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return identifiers, defaults, nil, nil
	}

	p.nextToken()

	// Check for *args
	if p.curTokenIs(token.ASTERISK) {
		// *args
		if !p.expectPeek(token.IDENT) {
			return nil, nil, nil, nil
		}
		variadic = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		// Check for **kwargs after *args
		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // consume comma
			if p.peekTokenIs(token.POW) {
				p.nextToken() // consume POW (**)
				if !p.expectPeek(token.IDENT) {
					return nil, nil, nil, nil
				}
				kwargs = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
			}
		}
		if !p.expectPeek(token.RPAREN) {
			return nil, nil, nil, nil
		}
		return identifiers, defaults, variadic, kwargs
	}

	// Check for **kwargs at start
	if p.curTokenIs(token.POW) {
		if !p.expectPeek(token.IDENT) {
			return nil, nil, nil, nil
		}
		kwargs = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		if !p.expectPeek(token.RPAREN) {
			return nil, nil, nil, nil
		}
		return identifiers, defaults, variadic, kwargs
	}

	ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	identifiers = append(identifiers, ident)

	// Check for default value
	if p.peekTokenIs(token.ASSIGN) {
		if defaults == nil {
			defaults = make(map[string]ast.Expression, 2)
		}
		p.nextToken() // consume =
		p.nextToken()
		defaults[ident.Value] = p.parseExpression(LOWEST)
	}

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		if p.peekTokenIs(token.RPAREN) {
			break
		}
		p.nextToken()

		// Check for *args
		if p.curTokenIs(token.ASTERISK) {
			// *args
			if !p.expectPeek(token.IDENT) {
				return nil, nil, nil, nil
			}
			variadic = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
			// Check for **kwargs after *args
			if p.peekTokenIs(token.COMMA) {
				p.nextToken() // consume comma
				if p.peekTokenIs(token.POW) {
					p.nextToken() // consume POW (**)
					if !p.expectPeek(token.IDENT) {
						return nil, nil, nil, nil
					}
					kwargs = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
				}
			}
			if !p.expectPeek(token.RPAREN) {
				return nil, nil, nil, nil
			}
			return identifiers, defaults, variadic, kwargs
		}

		// Check for **kwargs
		if p.curTokenIs(token.POW) {
			if !p.expectPeek(token.IDENT) {
				return nil, nil, nil, nil
			}
			kwargs = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
			if !p.expectPeek(token.RPAREN) {
				return nil, nil, nil, nil
			}
			return identifiers, defaults, variadic, kwargs
		}

		ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		identifiers = append(identifiers, ident)

		// Check for default value
		if p.peekTokenIs(token.ASSIGN) {
			if defaults == nil {
				defaults = make(map[string]ast.Expression, 2)
			}
			p.nextToken() // consume =
			p.nextToken()
			defaults[ident.Value] = p.parseExpression(LOWEST)
		}
	}

	if !p.expectPeek(token.RPAREN) {
		return nil, nil, nil, nil
	}

	return identifiers, defaults, variadic, kwargs
}

func (p *Parser) parseForStatement() *ast.ForStatement {
	stmt := &ast.ForStatement{Token: p.curToken}

	p.nextToken() // move to first variable

	// Parse the variable list (can be single or multiple separated by commas)
	stmt.Variables = make([]ast.Expression, 0, 2)
	stmt.Variables = append(stmt.Variables, p.parseExpression(EQUALS))

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // consume comma
		p.nextToken() // move to next expression
		stmt.Variables = append(stmt.Variables, p.parseExpression(EQUALS))
	}

	if !p.expectPeek(token.IN) {
		return nil
	}

	p.nextToken()
	stmt.Iterable = p.parseExpression(LOWEST)

	if !p.expectPeek(token.COLON) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()

	if p.peekTokenIs(token.ELSE) {
		p.nextToken()
		if !p.expectPeek(token.COLON) {
			return nil
		}
		stmt.Else = p.parseBlockStatement()
	}

	return stmt
}

func (p *Parser) parseListLiteral() ast.Expression {
	list := &ast.ListLiteral{Token: p.curToken}

	// Check for empty list
	if p.peekTokenIs(token.RBRACKET) {
		p.nextToken()
		return list
	}

	p.nextToken()
	firstExpr := p.parseExpression(LOWEST)

	// Check if this is a list comprehension
	if p.peekTokenIs(token.FOR) {
		return p.parseListComprehension(firstExpr)
	}

	// Regular list literal
	elements := make([]ast.Expression, 1, 4)
	elements[0] = firstExpr

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		if p.peekTokenIs(token.RBRACKET) {
			break
		}
		p.nextToken()
		elements = append(elements, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(token.RBRACKET) {
		return nil
	}

	list.Elements = elements
	return list
}

func (p *Parser) parseListComprehension(expr ast.Expression) ast.Expression {
	return p.parseComprehensionCore(expr, token.RBRACKET)
}

// parseGeneratorExpression parses generator expressions like (x for x in list)
// We treat them as list comprehensions for simplicity (eager evaluation)
func (p *Parser) parseGeneratorExpression(expr ast.Expression) ast.Expression {
	return p.parseComprehensionCore(expr, token.RPAREN)
}

// parseGeneratorExpressionInCall parses generator expressions in function calls
// like: func(x for x in list) - without surrounding parens
func (p *Parser) parseGeneratorExpressionInCall(expr ast.Expression, end token.TokenType) ast.Expression {
	return p.parseComprehensionCore(expr, end)
}

// parseAdditionalClauses parses zero or more additional `for var in iter [if cond]` clauses
func (p *Parser) parseAdditionalClauses() []ast.ComprehensionClause {
	var clauses []ast.ComprehensionClause
	for p.peekTokenIs(token.FOR) {
		p.nextToken() // consume FOR
		p.nextToken() // move to variable
		vars := make([]ast.Expression, 1, 2)
		vars[0] = p.parseExpression(EQUALS)
		for p.peekTokenIs(token.COMMA) {
			p.nextToken()
			p.nextToken()
			vars = append(vars, p.parseExpression(EQUALS))
		}
		if !p.expectPeek(token.IN) {
			return nil
		}
		p.nextToken()
		iter := p.parseExpression(CONDITIONAL)
		var cond ast.Expression
		if p.peekTokenIs(token.IF) {
			p.nextToken()
			p.nextToken()
			cond = p.parseExpression(CONDITIONAL)
		}
		clauses = append(clauses, ast.ComprehensionClause{Variables: vars, Iterable: iter, Condition: cond})
	}
	return clauses
}

// parseComprehensionCore is the unified implementation for list comprehensions and generator expressions
func (p *Parser) parseComprehensionCore(expr ast.Expression, endToken token.TokenType) ast.Expression {
	comp := &ast.ListComprehension{
		Token:      p.curToken,
		Expression: expr,
	}

	if !p.expectPeek(token.FOR) {
		return nil
	}

	p.nextToken()
	comp.Variables = make([]ast.Expression, 1, 2)
	comp.Variables[0] = p.parseExpression(EQUALS)
	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		comp.Variables = append(comp.Variables, p.parseExpression(EQUALS))
	}

	if !p.expectPeek(token.IN) {
		return nil
	}
	p.nextToken()
	comp.Iterable = p.parseExpression(CONDITIONAL)

	if p.peekTokenIs(token.IF) {
		p.nextToken()
		p.nextToken()
		comp.Condition = p.parseExpression(CONDITIONAL)
	}

	comp.AdditionalClauses = p.parseAdditionalClauses()

	if !p.expectPeek(endToken) {
		return nil
	}
	return comp
}

func (p *Parser) parseLambda() ast.Expression {
	lambda := &ast.Lambda{Token: p.curToken}

	// Parse parameters (optional)
	if !p.peekTokenIs(token.COLON) {
		lambda.Parameters, lambda.DefaultValues, lambda.Variadic, lambda.Kwargs = p.parseLambdaParameters()
	}

	if !p.expectPeek(token.COLON) {
		return nil
	}

	p.nextToken()
	lambda.Body = p.parseExpression(LOWEST)

	return lambda
}

func (p *Parser) parseLambdaParameters() ([]*ast.Identifier, map[string]ast.Expression, *ast.Identifier, *ast.Identifier) {
	var identifiers []*ast.Identifier
	var defaults map[string]ast.Expression
	var variadic *ast.Identifier
	var kwargs *ast.Identifier

	p.nextToken()

	if p.curTokenIs(token.COLON) {
		return identifiers, defaults, nil, nil
	}

	// Check for *args or **kwargs
	if p.curTokenIs(token.ASTERISK) {
		p.nextToken()
		variadic = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		// Check for **kwargs after *args
		if p.peekTokenIs(token.COMMA) {
			p.nextToken() // consume comma
			if p.peekTokenIs(token.POW) {
				p.nextToken() // consume POW (**)
				if !p.expectPeek(token.IDENT) {
					return nil, nil, nil, nil
				}
				kwargs = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
			}
		}
		return identifiers, defaults, variadic, kwargs
	}

	// Check for **kwargs at start
	if p.curTokenIs(token.POW) {
		if !p.expectPeek(token.IDENT) {
			return nil, nil, nil, nil
		}
		kwargs = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		return identifiers, defaults, variadic, kwargs
	}

	ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	identifiers = append(identifiers, ident)

	// Check for default value
	if p.peekTokenIs(token.ASSIGN) {
		if defaults == nil {
			defaults = make(map[string]ast.Expression, 2)
		}
		p.nextToken() // consume =
		p.nextToken()
		defaults[ident.Value] = p.parseExpression(LOWEST)
	}

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()

		// Check for *args or **kwargs
		if p.curTokenIs(token.ASTERISK) {
			p.nextToken()
			variadic = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
			// Check for **kwargs after *args
			if p.peekTokenIs(token.COMMA) {
				p.nextToken() // consume comma
				if p.peekTokenIs(token.POW) {
					p.nextToken() // consume POW (**)
					if !p.expectPeek(token.IDENT) {
						return nil, nil, nil, nil
					}
					kwargs = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
				}
			}
			return identifiers, defaults, variadic, kwargs
		}

		// Check for **kwargs
		if p.curTokenIs(token.POW) {
			if !p.expectPeek(token.IDENT) {
				return nil, nil, nil, nil
			}
			kwargs = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
			return identifiers, defaults, variadic, kwargs
		}

		ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		identifiers = append(identifiers, ident)

		// Check for default value
		if p.peekTokenIs(token.ASSIGN) {
			if defaults == nil {
				defaults = make(map[string]ast.Expression, 2)
			}
			p.nextToken() // consume =
			p.nextToken()
			defaults[ident.Value] = p.parseExpression(LOWEST)
		}
	}

	return identifiers, defaults, variadic, kwargs
}

func (p *Parser) skipWhitespace() {
	for p.peekTokenIs(token.NEWLINE) || p.peekTokenIs(token.INDENT) || p.peekTokenIs(token.DEDENT) {
		p.peekToken = p.l.NextToken()
	}
}

func (p *Parser) parseDictLiteral() ast.Expression {
	tok := p.curToken
	dict := &ast.DictLiteral{Token: tok}

	p.skipWhitespace()
	if p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		return dict
	}

	p.nextToken()

	for {
		// Skip whitespace before key
		for p.curTokenIs(token.NEWLINE) || p.curTokenIs(token.INDENT) || p.curTokenIs(token.DEDENT) {
			p.nextToken()
		}

		if p.curTokenIs(token.RBRACE) {
			return dict
		}

		first := p.parseExpression(LOWEST)

		// Peek past whitespace to determine dict vs set
		p.skipWhitespace()
		if p.peekTokenIs(token.FOR) {
			// Set comprehension: {expr for var in iterable}
			return p.parseSetComprehension(tok, first)
		}

		if p.peekTokenIs(token.COMMA) || p.peekTokenIs(token.RBRACE) {
			// Set literal: {expr, expr, ...} or {expr}
			return p.parseSetLiteralFrom(tok, first)
		}

		p.skipWhitespace()
		if !p.expectPeek(token.COLON) {
			return nil
		}

		p.nextToken()
		// Skip whitespace after colon
		for p.curTokenIs(token.NEWLINE) || p.curTokenIs(token.INDENT) || p.curTokenIs(token.DEDENT) {
			p.nextToken()
		}

		value := p.parseExpression(LOWEST)

		// Check for dict comprehension: {k: v for ...}
		if p.peekTokenIs(token.FOR) {
			return p.parseDictComprehension(tok, first, value)
		}

		dict.Pairs = append(dict.Pairs, ast.DictPairLiteral{
			Key:   first,
			Value: value,
		})

		p.skipWhitespace()
		if !p.peekTokenIs(token.COMMA) {
			break
		}
		p.nextToken()
		p.skipWhitespace()
		if p.peekTokenIs(token.RBRACE) {
			break
		}
		p.nextToken()
	}

	p.skipWhitespace()
	if !p.expectPeek(token.RBRACE) {
		return nil
	}

	return dict
}

func (p *Parser) parseDictComprehension(tok token.Token, keyExpr, valueExpr ast.Expression) ast.Expression {
	comp := &ast.DictComprehension{
		Token: tok,
		Key:   keyExpr,
		Value: valueExpr,
	}

	if !p.expectPeek(token.FOR) {
		return nil
	}

	p.nextToken()
	comp.Variables = make([]ast.Expression, 1, 2)
	comp.Variables[0] = p.parseExpression(EQUALS)
	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		comp.Variables = append(comp.Variables, p.parseExpression(EQUALS))
	}

	if !p.expectPeek(token.IN) {
		return nil
	}
	p.nextToken()
	comp.Iterable = p.parseExpression(CONDITIONAL)

	if p.peekTokenIs(token.IF) {
		p.nextToken()
		p.nextToken()
		comp.Condition = p.parseExpression(CONDITIONAL)
	}

	comp.AdditionalClauses = p.parseAdditionalClauses()

	if !p.expectPeek(token.RBRACE) {
		return nil
	}
	return comp
}

func (p *Parser) parseSetComprehension(tok token.Token, expr ast.Expression) ast.Expression {
	comp := &ast.SetComprehension{
		Token:      tok,
		Expression: expr,
	}

	if !p.expectPeek(token.FOR) {
		return nil
	}

	p.nextToken()
	comp.Variables = make([]ast.Expression, 1, 2)
	comp.Variables[0] = p.parseExpression(EQUALS)
	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		comp.Variables = append(comp.Variables, p.parseExpression(EQUALS))
	}

	if !p.expectPeek(token.IN) {
		return nil
	}
	p.nextToken()
	comp.Iterable = p.parseExpression(CONDITIONAL)

	if p.peekTokenIs(token.IF) {
		p.nextToken()
		p.nextToken()
		comp.Condition = p.parseExpression(CONDITIONAL)
	}

	comp.AdditionalClauses = p.parseAdditionalClauses()

	if !p.expectPeek(token.RBRACE) {
		return nil
	}
	return comp
}

func (p *Parser) parseSetLiteralFrom(tok token.Token, first ast.Expression) ast.Expression {
	set := &ast.SetLiteral{Token: tok, Elements: make([]ast.Expression, 1, 4)}
	set.Elements[0] = first

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // consume comma
		p.skipWhitespace()
		if p.peekTokenIs(token.RBRACE) {
			break
		}
		p.nextToken()
		for p.curTokenIs(token.NEWLINE) || p.curTokenIs(token.INDENT) || p.curTokenIs(token.DEDENT) {
			p.nextToken()
		}
		set.Elements = append(set.Elements, p.parseExpression(LOWEST))
	}

	p.skipWhitespace()
	if !p.expectPeek(token.RBRACE) {
		return nil
	}
	return set
}

func (p *Parser) parseTryStatement() *ast.TryStatement {
	stmt := &ast.TryStatement{Token: p.curToken}

	if !p.expectPeek(token.COLON) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()

	// Parse except clauses (can have multiple)
	for p.peekTokenIs(token.EXCEPT) {
		if stmt.ExceptClauses == nil {
			stmt.ExceptClauses = make([]*ast.ExceptClause, 0, 2)
		}
		p.nextToken() // consume except
		exceptClause := &ast.ExceptClause{Token: p.curToken}

		// Support bare except, single exception types, dotted names, and tuples
		// like `except (TypeError, ValueError):`.
		if !p.peekTokenIs(token.COLON) {
			p.nextToken()
			exceptClause.ExceptType = p.parseExpression(LOWEST)
			if p.peekTokenIs(token.AS) {
				p.nextToken() // consume 'as'
				if p.expectPeek(token.IDENT) {
					exceptClause.ExceptVar = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
				}
			}
		}

		if !p.expectPeek(token.COLON) {
			return nil
		}
		exceptClause.Body = p.parseBlockStatement()
		stmt.ExceptClauses = append(stmt.ExceptClauses, exceptClause)
	}

	// Parse else clause (optional, runs only when no exception was raised)
	if p.peekTokenIs(token.ELSE) {
		p.nextToken()
		if !p.expectPeek(token.COLON) {
			return nil
		}
		stmt.Else = p.parseBlockStatement()
	}

	// Parse finally clause (optional)
	if p.peekTokenIs(token.FINALLY) {
		p.nextToken()
		if !p.expectPeek(token.COLON) {
			return nil
		}
		stmt.Finally = p.parseBlockStatement()
	}

	return stmt
}

func (p *Parser) parseRaiseStatement() *ast.RaiseStatement {
	stmt := &ast.RaiseStatement{Token: p.curToken}

	if !p.peekTokenIs(token.NEWLINE) && !p.peekTokenIs(token.SEMICOLON) && !p.peekTokenIs(token.EOF) && !p.peekTokenIs(token.DEDENT) {
		p.nextToken()
		stmt.Message = p.parseExpression(LOWEST)
	}

	return stmt
}

func (p *Parser) parseGlobalStatement() *ast.GlobalStatement {
	stmt := &ast.GlobalStatement{Token: p.curToken}
	stmt.Names = make([]*ast.Identifier, 0, 2)

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	stmt.Names = append(stmt.Names, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // consume comma
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		stmt.Names = append(stmt.Names, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})
	}

	return stmt
}

func (p *Parser) parseNonlocalStatement() *ast.NonlocalStatement {
	stmt := &ast.NonlocalStatement{Token: p.curToken}
	stmt.Names = make([]*ast.Identifier, 0, 2)

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	stmt.Names = append(stmt.Names, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // consume comma
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		stmt.Names = append(stmt.Names, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})
	}

	return stmt
}

func (p *Parser) parseDecoratedStatement() ast.Statement {
	// Collect all decorator expressions
	var decorators []ast.Expression
	for p.curTokenIs(token.AT) {
		p.nextToken() // consume '@', move to decorator name
		dec := p.parseExpression(LOWEST)
		if dec == nil {
			return nil
		}
		decorators = append(decorators, dec)
		// nextToken is called by ParseProgram/parseBlockStatement after each statement,
		// but here we need to advance past the decorator line manually.
		// After parseExpression the curToken is the last token of the decorator.
		// The NEWLINE was already consumed by nextToken's skip logic, so peekToken
		// should be AT, DEF, or CLASS.
		p.nextToken()
	}
	switch p.curToken.Type {
	case token.DEF:
		stmt := p.parseFunctionStatement()
		if stmt != nil {
			stmt.Decorators = decorators
		}
		return stmt
	case token.CLASS:
		stmt := p.parseClassStatement()
		if stmt != nil {
			stmt.Decorators = decorators
		}
		return stmt
	default:
		p.errors = append(p.errors, fmt.Sprintf("line %d: expected def or class after decorator, got %s", p.curToken.Line, p.curToken.Type))
		return nil
	}
}

func (p *Parser) parseWithStatement() *ast.WithStatement {
	stmt := &ast.WithStatement{Token: p.curToken}
	p.nextToken()
	stmt.ContextExpr = p.parseExpression(LOWEST)
	if p.peekTokenIs(token.AS) {
		p.nextToken() // consume 'as'
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		stmt.Target = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}
	if !p.expectPeek(token.COLON) {
		return nil
	}
	stmt.Body = p.parseBlockStatement()
	return stmt
}

func (p *Parser) parseAssertStatement() *ast.AssertStatement {
	stmt := &ast.AssertStatement{Token: p.curToken}
	p.nextToken()

	// Parse the condition expression
	stmt.Condition = p.parseExpression(LOWEST)

	// Check for optional message after comma
	if p.peekTokenIs(token.COMMA) {
		p.nextToken() // consume comma
		p.nextToken() // move to the message expression
		stmt.Message = p.parseExpression(LOWEST)
	}

	return stmt
}

func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	if p.curTokenIs(token.DOT) {
		// Method call or member access: obj.method() or obj.member
		// Allow keywords as attribute names (e.g., re.match, obj.class)
		p.nextToken()
		if p.curToken.Type != token.IDENT && !p.isKeyword(p.curToken.Type) {
			p.errors = append(p.errors, fmt.Sprintf("line %d: expected identifier after '.', got %s", p.curToken.Line, p.curToken.Type))
			return nil
		}
		methodName := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

		// Check if this is a method call (followed by parentheses)
		if p.peekTokenIs(token.LPAREN) {
			p.nextToken() // consume LPAREN
			methodCall := &ast.MethodCallExpression{
				Token:  p.curToken,
				Object: left,
				Method: methodName,
			}
			methodCall.Arguments, methodCall.Keywords, methodCall.ArgsUnpack, methodCall.KwargsUnpack = p.parseCallArguments()
			return methodCall
		}

		// Regular member access: obj.member
		exp := &ast.IndexExpression{Token: p.curToken, Left: left, IsDotAccess: true}
		exp.Index = &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
		return exp
	}

	// Bracket access: obj[index] or obj[start:end]
	tok := p.curToken
	p.nextToken()

	// Check for slice notation
	if p.curTokenIs(token.COLON) {
		// Slice with no start: [:end] or [:end:step]
		slice := &ast.SliceExpression{Token: tok, Left: left, Start: nil}
		if !p.peekTokenIs(token.RBRACKET) && !p.peekTokenIs(token.COLON) {
			p.nextToken()
			slice.End = p.parseExpression(LOWEST)
		}
		// Check for step parameter
		if p.peekTokenIs(token.COLON) {
			p.nextToken() // consume second colon
			if !p.peekTokenIs(token.RBRACKET) {
				p.nextToken()
				slice.Step = p.parseExpression(LOWEST)
			}
		}
		if !p.expectPeek(token.RBRACKET) {
			return nil
		}
		return slice
	}

	start := p.parseExpression(LOWEST)

	if p.peekTokenIs(token.COLON) {
		// Slice notation: [start:end] or [start:end:step]
		p.nextToken() // consume colon
		slice := &ast.SliceExpression{Token: tok, Left: left, Start: start}
		if !p.peekTokenIs(token.RBRACKET) && !p.peekTokenIs(token.COLON) {
			p.nextToken()
			slice.End = p.parseExpression(LOWEST)
		}
		// Check for step parameter
		if p.peekTokenIs(token.COLON) {
			p.nextToken() // consume second colon
			if !p.peekTokenIs(token.RBRACKET) {
				p.nextToken()
				slice.Step = p.parseExpression(LOWEST)
			}
		}
		if !p.expectPeek(token.RBRACKET) {
			return nil
		}
		return slice
	}

	// Regular index: [index]
	exp := &ast.IndexExpression{Token: tok, Left: left, Index: start}
	if !p.expectPeek(token.RBRACKET) {
		return nil
	}
	return exp
}

func (p *Parser) parseMatchStatement() *ast.MatchStatement {
	stmt := &ast.MatchStatement{Token: p.curToken}

	p.nextToken()
	stmt.Subject = p.parseExpression(LOWEST_PRECEDENCE)

	if !p.expectPeek(token.COLON) {
		return nil
	}

	// Expect INDENT (parseBlockStatement-style, which handles NEWLINE internally via expectPeek)
	if !p.expectPeek(token.INDENT) {
		return nil
	}

	p.nextToken()

	// Parse case clauses
	for p.curTokenIs(token.IDENT) && p.curToken.Literal == "case" {
		if stmt.Cases == nil {
			stmt.Cases = make([]*ast.CaseClause, 0, 2)
		}
		caseClause := p.parseCaseClause()
		if caseClause == nil {
			return nil
		}
		stmt.Cases = append(stmt.Cases, caseClause)
		p.nextToken()
	}

	if len(stmt.Cases) == 0 {
		p.errors = append(p.errors, "match statement must have at least one case clause")
		return nil
	}

	// Current token should be DEDENT
	if !p.curTokenIs(token.DEDENT) {
		p.errors = append(p.errors, "expected DEDENT after match cases")
		return nil
	}

	return stmt
}

func (p *Parser) parseCaseClause() *ast.CaseClause {
	clause := &ast.CaseClause{Token: p.curToken}

	p.nextToken()
	// Parse pattern - stop before 'if' (guard) or 'as' (capture) or ':'
	clause.Pattern = p.parseCasePattern()

	// Check for 'as' capture variable
	if p.peekTokenIs(token.AS) {
		p.nextToken() // consume 'as'
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		clause.CaptureAs = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}

	// Check for guard condition (if clause)
	if p.peekTokenIs(token.IF) {
		p.nextToken() // consume 'if'
		p.nextToken()
		clause.Guard = p.parseExpression(LOWEST_PRECEDENCE)
	}

	if !p.expectPeek(token.COLON) {
		return nil
	}

	clause.Body = p.parseBlockStatement()

	return clause
}

// parseCasePattern parses a pattern in a case clause, stopping at 'if', 'as', or ':'
func (p *Parser) parseCasePattern() ast.Expression {
	// We need to parse an expression but stop at 'if' or 'as' keywords
	// These are used for guards and captures, not part of the pattern itself
	// Use BIT_OR precedence to stop before '|' (used for OR patterns)
	var first ast.Expression
	switch p.curToken.Type {
	case token.IDENT:
		// Check if next token is 'if' or 'as' - if so, this is just an identifier
		if p.peekTokenIs(token.IF) || p.peekTokenIs(token.AS) || p.peekTokenIs(token.COLON) || p.peekTokenIs(token.PIPE) {
			first = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		} else {
			// Otherwise parse as normal expression, stopping before '|'
			first = p.parseExpression(BIT_OR)
		}
	default:
		first = p.parseExpression(BIT_OR)
	}

	// Check for OR pattern: case 1 | 2 | 3
	if p.peekTokenIs(token.PIPE) {
		patterns := make([]ast.Expression, 1, 4)
		patterns[0] = first
		for p.peekTokenIs(token.PIPE) {
			p.nextToken() // consume '|'
			p.nextToken() // move to next pattern
			patterns = append(patterns, p.parseExpression(BIT_OR))
		}
		return &ast.OrPattern{Token: p.curToken, Patterns: patterns}
	}
	return first
}

func (p *Parser) isKeyword(t token.TokenType) bool {
	switch t {
	case token.TRUE, token.FALSE, token.NONE, token.IMPORT, token.FROM,
		token.IF, token.ELIF, token.ELSE, token.WHILE, token.FOR, token.IN,
		token.DEF, token.CLASS, token.RETURN, token.BREAK, token.CONTINUE,
		token.PASS, token.DEL, token.AND, token.OR, token.NOT, token.IS, token.TRY,
		token.EXCEPT, token.FINALLY, token.RAISE, token.GLOBAL, token.NONLOCAL,
		token.LAMBDA, token.AS, token.ASSERT, token.WITH:
		return true
	default:
		return false
	}
}
