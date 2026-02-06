package parser

import (
	"fmt"
	"strconv"

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

var precedences = map[token.TokenType]int{
	token.IF:        CONDITIONAL,
	token.OR:        OR,
	token.PIPE:      BIT_OR,
	token.CARET:     BIT_XOR,
	token.AMPERSAND: BIT_AND,
	token.AND:       AND,
	token.EQ:        EQUALS,
	token.NOT_EQ:    EQUALS,
	token.IN:        EQUALS,
	token.NOT_IN:    EQUALS,
	token.IS:        EQUALS,
	token.IS_NOT:    EQUALS,
	token.LT:        LESSGREATER,
	token.GT:        LESSGREATER,
	token.LTE:       LESSGREATER,
	token.GTE:       LESSGREATER,
	token.LSHIFT:    BIT_SHIFT,
	token.RSHIFT:    BIT_SHIFT,
	token.PLUS:      SUM,
	token.MINUS:     SUM,
	token.SLASH:     PRODUCT,
	token.FLOORDIV:  PRODUCT,
	token.ASTERISK:  PRODUCT,
	token.PERCENT:   PRODUCT,
	token.POW:       POWER,
	token.LPAREN:    CALL,
	token.LBRACKET:  CALL,
	token.DOT:       CALL,
}

type Parser struct {
	l      *lexer.Lexer
	errors []string

	curToken       token.Token
	peekToken      token.Token
	skippedNewline bool // true if a NEWLINE was skipped between curToken and peekToken
	parenDepth     int  // track parenthesis depth for multiline support

	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l, errors: []string{}}

	p.prefixParseFns = make(map[token.TokenType]prefixParseFn)
	p.registerPrefix(token.IDENT, p.parseIdentifier)
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.FLOAT, p.parseFloatLiteral)
	p.registerPrefix(token.STRING, p.parseStringLiteral)
	p.registerPrefix(token.F_STRING, p.parseFStringLiteral)
	p.registerPrefix(token.TRUE, p.parseBoolean)
	p.registerPrefix(token.FALSE, p.parseBoolean)
	p.registerPrefix(token.NONE, p.parseNone)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)
	p.registerPrefix(token.NOT, p.parsePrefixExpression)
	p.registerPrefix(token.TILDE, p.parsePrefixExpression)
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(token.LBRACKET, p.parseListLiteral)
	p.registerPrefix(token.LBRACE, p.parseDictLiteral)
	p.registerPrefix(token.LAMBDA, p.parseLambda)

	p.infixParseFns = make(map[token.TokenType]infixParseFn)
	p.registerInfix(token.PLUS, p.parseInfixExpression)
	p.registerInfix(token.MINUS, p.parseInfixExpression)
	p.registerInfix(token.SLASH, p.parseInfixExpression)
	p.registerInfix(token.FLOORDIV, p.parseInfixExpression)
	p.registerInfix(token.ASTERISK, p.parseInfixExpression)
	p.registerInfix(token.POW, p.parseInfixExpression)
	p.registerInfix(token.PERCENT, p.parseInfixExpression)
	p.registerInfix(token.EQ, p.parseInfixExpression)
	p.registerInfix(token.NOT_EQ, p.parseInfixExpression)
	p.registerInfix(token.LT, p.parseInfixExpression)
	p.registerInfix(token.GT, p.parseInfixExpression)
	p.registerInfix(token.LTE, p.parseInfixExpression)
	p.registerInfix(token.GTE, p.parseInfixExpression)
	p.registerInfix(token.AND, p.parseInfixExpression)
	p.registerInfix(token.OR, p.parseInfixExpression)
	p.registerInfix(token.IN, p.parseInfixExpression)
	p.registerInfix(token.NOT_IN, p.parseInfixExpression)
	p.registerInfix(token.IS, p.parseInfixExpression)
	p.registerInfix(token.IS_NOT, p.parseInfixExpression)
	p.registerInfix(token.AMPERSAND, p.parseInfixExpression)
	p.registerInfix(token.PIPE, p.parseInfixExpression)
	p.registerInfix(token.CARET, p.parseInfixExpression)
	p.registerInfix(token.LSHIFT, p.parseInfixExpression)
	p.registerInfix(token.RSHIFT, p.parseInfixExpression)
	p.registerInfix(token.LPAREN, p.parseCallExpression)
	p.registerInfix(token.LBRACKET, p.parseIndexExpression)
	p.registerInfix(token.DOT, p.parseIndexExpression)
	p.registerInfix(token.IF, p.parseConditionalExpression)

	p.nextToken()
	p.nextToken()

	return p
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

func (p *Parser) registerPrefix(tokenType token.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType token.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	program.Statements = []ast.Statement{}

	for !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.NEWLINE) || p.curTokenIs(token.SEMICOLON) {
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
	case token.MATCH:
		return p.parseMatchStatement()
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
	case token.IDENT:
		if p.peekTokenIs(token.ASSIGN) {
			return p.parseAssignStatement()
		} else if p.peekTokenIs(token.COMMA) {
			return p.parseMultipleAssignStatement()
		} else if p.isAugmentedAssign() {
			return p.parseAugmentedAssignStatement()
		}
		return p.parseExpressionStatement()
	default:
		return p.parseExpressionStatement()
	}
}

func (p *Parser) isAugmentedAssign() bool {
	return p.peekTokenIs(token.PLUS_EQ) || p.peekTokenIs(token.MINUS_EQ) ||
		p.peekTokenIs(token.MUL_EQ) || p.peekTokenIs(token.DIV_EQ) || p.peekTokenIs(token.FLOORDIV_EQ) ||
		p.peekTokenIs(token.MOD_EQ) ||
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
	stmt.Value = p.parseExpressionWithConditional()

	return stmt
}

func (p *Parser) parseMultipleAssignStatement() ast.Statement {
	names := []*ast.Identifier{}
	names = append(names, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})

	// Parse remaining identifiers
	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // consume comma
		if !p.expectPeek(token.IDENT) {
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
		values := []ast.Expression{firstValue}
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
			Token: names[0].Token,
			Names: names,
			Value: value,
		}
	}

	// Single value (must be a tuple/list to unpack)
	return &ast.MultipleAssignStatement{
		Token: names[0].Token,
		Names: names,
		Value: firstValue,
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

	// Build up the dotted name (e.g., urllib.parse)
	name := p.curToken.Literal
	for p.peekTokenIs(token.DOT) {
		p.nextToken() // consume dot
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		name = name + "." + p.curToken.Literal
	}

	stmt.Name = &ast.Identifier{Token: p.curToken, Value: name}

	// Check for alias (as)
	if p.peekTokenIs(token.AS) {
		p.nextToken() // consume 'as'
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		stmt.Alias = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}

	// Check for additional imports separated by commas
	stmt.AdditionalNames = []*ast.Identifier{}
	stmt.AdditionalAliases = []*ast.Identifier{}
	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // consume comma
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		// Build up the dotted name for additional imports too
		addName := p.curToken.Literal
		for p.peekTokenIs(token.DOT) {
			p.nextToken() // consume dot
			if !p.expectPeek(token.IDENT) {
				return nil
			}
			addName = addName + "." + p.curToken.Literal
		}
		stmt.AdditionalNames = append(stmt.AdditionalNames, &ast.Identifier{
			Token: p.curToken,
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
func (p *Parser) parseFromImportStatement() *ast.FromImportStatement {
	stmt := &ast.FromImportStatement{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	// Build up the dotted module name (e.g., urllib.parse)
	moduleName := p.curToken.Literal
	for p.peekTokenIs(token.DOT) {
		p.nextToken() // consume dot
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		moduleName = moduleName + "." + p.curToken.Literal
	}

	stmt.Module = &ast.Identifier{Token: p.curToken, Value: moduleName}

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

func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	stmt := &ast.ReturnStatement{Token: p.curToken}
	p.nextToken()

	if !p.curTokenIs(token.NEWLINE) && !p.curTokenIs(token.SEMICOLON) && !p.curTokenIs(token.EOF) {
		stmt.ReturnValue = p.parseExpressionWithConditional()
	}

	return stmt
}

func (p *Parser) parseExpressionStatement() ast.Statement {
	expr := p.parseExpressionWithConditional()
	if p.peekTokenIs(token.ASSIGN) {
		// It's an assignment
		stmt := &ast.AssignStatement{Token: p.curToken, Left: expr}
		p.nextToken() // consume =
		p.nextToken() // move to value
		stmt.Value = p.parseExpressionWithConditional()
		return stmt
	}
	return &ast.ExpressionStatement{Token: p.curToken, Expression: expr}
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	leftExp := prefix()

	for !p.peekTokenIs(token.NEWLINE) && !p.peekTokenIs(token.SEMICOLON) && !p.peekTokenIs(token.EOF) && !p.peekTokenIs(token.COLON) && precedence < p.peekPrecedence() {
		// Special handling for IF token: only treat as conditional expression
		// if it appears on the same line (no newline was skipped)
		if p.peekTokenIs(token.IF) && p.skippedNewline {
			return leftExp
		}
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}
		p.nextToken()
		leftExp = infix(leftExp)
	}

	return leftExp
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

func (p *Parser) noPrefixParseFnError(t token.TokenType) {
	msg := fmt.Sprintf("line %d: no prefix parse function for %s found", p.curToken.Line, t)
	p.errors = append(p.errors, msg)
}

func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	lit := &ast.IntegerLiteral{Token: p.curToken}
	value, err := strconv.ParseInt(p.curToken.Literal, 0, 64)
	if err != nil {
		msg := fmt.Sprintf("line %d: could not parse %q as integer", p.curToken.Line, p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}
	lit.Value = value
	return lit
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
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseFStringLiteral() ast.Expression {
	fstr := &ast.FStringLiteral{Token: p.curToken}
	fstr.Parts, fstr.Expressions, fstr.FormatSpecs = p.parseFStringContent(p.curToken.Literal)
	return fstr
}

func (p *Parser) parseFStringContent(content string) ([]string, []ast.Expression, []string) {
	parts := []string{}
	expressions := []ast.Expression{}
	formatSpecs := []string{}
	current := ""
	i := 0

	for i < len(content) {
		if content[i] == '{' && i+1 < len(content) && content[i+1] != '{' {
			// Found expression start
			parts = append(parts, current)
			current = ""
			i++ // skip {

			// Extract expression until : or }
			exprStr := ""
			formatSpec := ""
			for i < len(content) && content[i] != '}' && content[i] != ':' {
				exprStr += string(content[i])
				i++
			}

			// Check for format specifier
			if i < len(content) && content[i] == ':' {
				i++ // skip :
				// Extract format spec until }
				for i < len(content) && content[i] != '}' {
					formatSpec += string(content[i])
					i++
				}
			}

			if i < len(content) {
				i++ // skip }
			}

			// Parse the expression
			if exprStr != "" {
				lexer := lexer.New(exprStr)
				parser := New(lexer)
				expr := parser.parseExpression(LOWEST)
				if expr != nil {
					expressions = append(expressions, expr)
					formatSpecs = append(formatSpecs, formatSpec)
				}
			}
		} else if content[i] == '{' && i+1 < len(content) && content[i+1] == '{' {
			// Escaped brace
			current += "{"
			i += 2
		} else if content[i] == '}' && i+1 < len(content) && content[i+1] == '}' {
			// Escaped brace
			current += "}"
			i += 2
		} else {
			current += string(content[i])
			i++
		}
	}

	parts = append(parts, current)
	return parts, expressions, formatSpecs
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
		return &ast.TupleLiteral{Token: p.curToken, Elements: []ast.Expression{}}
	}

	firstExp := p.parseExpression(LOWEST_PRECEDENCE)

	// Check if this is a generator expression (similar to list comprehension)
	if p.peekTokenIs(token.FOR) {
		return p.parseGeneratorExpression(firstExp)
	}

	// Check if this is a tuple (has comma)
	if p.peekTokenIs(token.COMMA) {
		elements := []ast.Expression{firstExp}

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
	args := []ast.Expression{}
	keywords := make(map[string]ast.Expression)
	argsUnpack := []ast.Expression{}
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
	stmt.ElifClauses = []*ast.ElifClause{}
	for p.peekTokenIs(token.ELIF) {
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

	return stmt
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

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
		if p.curTokenIs(token.NEWLINE) || p.curTokenIs(token.SEMICOLON) {
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
	identifiers := []*ast.Identifier{}
	defaults := make(map[string]ast.Expression)
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
	stmt.Variables = []ast.Expression{}
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
	elements := []ast.Expression{firstExpr}

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

// parseComprehensionCore is the unified implementation for list comprehensions and generator expressions
func (p *Parser) parseComprehensionCore(expr ast.Expression, endToken token.TokenType) ast.Expression {
	comp := &ast.ListComprehension{
		Token:      p.curToken,
		Expression: expr,
	}

	if !p.expectPeek(token.FOR) {
		return nil
	}

	// Parse variable(s) - supports tuple unpacking like: for h, t in ...
	p.nextToken() // move to first variable
	comp.Variables = []ast.Expression{}
	comp.Variables = append(comp.Variables, p.parseExpression(EQUALS))

	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // consume comma
		p.nextToken() // move to next expression
		comp.Variables = append(comp.Variables, p.parseExpression(EQUALS))
	}

	if !p.expectPeek(token.IN) {
		return nil
	}

	p.nextToken()
	// Use CONDITIONAL precedence to prevent 'if' from being consumed as conditional expression
	// In list comprehensions, 'if' is the filter keyword, not conditional expression
	comp.Iterable = p.parseExpression(CONDITIONAL)

	// Check for optional if condition
	if p.peekTokenIs(token.IF) {
		p.nextToken()
		p.nextToken()
		// The condition also shouldn't consume 'if' as conditional (though unlikely)
		comp.Condition = p.parseExpression(CONDITIONAL)
	}

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
	} else {
		lambda.Parameters = []*ast.Identifier{}
		lambda.DefaultValues = make(map[string]ast.Expression)
	}

	if !p.expectPeek(token.COLON) {
		return nil
	}

	p.nextToken()
	lambda.Body = p.parseExpression(LOWEST)

	return lambda
}

func (p *Parser) parseLambdaParameters() ([]*ast.Identifier, map[string]ast.Expression, *ast.Identifier, *ast.Identifier) {
	identifiers := []*ast.Identifier{}
	defaults := make(map[string]ast.Expression)
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
	dict := &ast.DictLiteral{Token: p.curToken}
	dict.Pairs = make(map[ast.Expression]ast.Expression)

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

		key := p.parseExpression(LOWEST)

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

		dict.Pairs[key] = value

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

func (p *Parser) parseTryStatement() *ast.TryStatement {
	stmt := &ast.TryStatement{Token: p.curToken}

	if !p.expectPeek(token.COLON) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()

	// Parse except clause (optional)
	if p.peekTokenIs(token.EXCEPT) {
		p.nextToken()

		// Check for 'except Exception as e:' or 'except module.Exception as e:' syntax
		if p.peekTokenIs(token.IDENT) {
			p.nextToken() // consume exception type (or module name)

			// Handle dotted exception names like requests.RequestException
			for p.peekTokenIs(token.DOT) {
				p.nextToken() // consume '.'
				if !p.expectPeek(token.IDENT) {
					return nil
				}
			}

			if p.peekTokenIs(token.AS) {
				p.nextToken() // consume 'as'
				if p.expectPeek(token.IDENT) {
					stmt.ExceptVar = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
				}
			}
		}

		if !p.expectPeek(token.COLON) {
			return nil
		}
		stmt.Except = p.parseBlockStatement()
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
	p.nextToken()

	if !p.curTokenIs(token.NEWLINE) && !p.curTokenIs(token.SEMICOLON) && !p.curTokenIs(token.EOF) && !p.curTokenIs(token.DEDENT) {
		stmt.Message = p.parseExpression(LOWEST)
	}

	return stmt
}

func (p *Parser) parseGlobalStatement() *ast.GlobalStatement {
	stmt := &ast.GlobalStatement{Token: p.curToken}
	stmt.Names = []*ast.Identifier{}

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
	stmt.Names = []*ast.Identifier{}

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
		exp := &ast.IndexExpression{Token: p.curToken, Left: left}
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
	stmt.Cases = []*ast.CaseClause{}
	for p.curTokenIs(token.CASE) {
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
	switch p.curToken.Type {
	case token.IDENT:
		// Check if next token is 'if' or 'as' - if so, this is just an identifier
		if p.peekTokenIs(token.IF) || p.peekTokenIs(token.AS) || p.peekTokenIs(token.COLON) {
			return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		}
		// Otherwise parse as normal expression
		return p.parseExpression(LOWEST_PRECEDENCE)
	default:
		return p.parseExpression(LOWEST_PRECEDENCE)
	}
}


func (p *Parser) isKeyword(t token.TokenType) bool {
	keywords := []token.TokenType{
		token.TRUE, token.FALSE, token.NONE, token.IMPORT, token.FROM,
		token.IF, token.ELIF, token.ELSE, token.WHILE, token.FOR, token.IN,
		token.DEF, token.CLASS, token.RETURN, token.BREAK, token.CONTINUE,
		token.PASS, token.AND, token.OR, token.NOT, token.IS, token.TRY,
		token.EXCEPT, token.FINALLY, token.RAISE, token.GLOBAL, token.NONLOCAL,
		token.LAMBDA, token.AS, token.ASSERT, token.MATCH, token.CASE,
	}
	for _, kw := range keywords {
		if t == kw {
			return true
		}
	}
	return false
}
