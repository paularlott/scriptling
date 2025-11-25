package parser

import (
	"fmt"
	"strconv"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/token"
)

const (
	_ int = iota
	LOWEST
	OR
	AND
	EQUALS
	LESSGREATER
	SUM
	PRODUCT
	POWER
	PREFIX
	CALL
)

var precedences = map[token.TokenType]int{
	token.OR:       OR,
	token.AND:      AND,
	token.EQ:       EQUALS,
	token.NOT_EQ:   EQUALS,
	token.IN:       EQUALS,
	token.NOT_IN:   EQUALS,
	token.LT:       LESSGREATER,
	token.GT:       LESSGREATER,
	token.LTE:      LESSGREATER,
	token.GTE:      LESSGREATER,
	token.PLUS:     SUM,
	token.MINUS:    SUM,
	token.SLASH:    PRODUCT,
	token.ASTERISK: PRODUCT,
	token.PERCENT:  PRODUCT,
	token.POW:      POWER,
	token.LPAREN:   CALL,
	token.LBRACKET: CALL,
	token.DOT:      CALL,
}

type Parser struct {
	l      *lexer.Lexer
	errors []string

	curToken  token.Token
	peekToken token.Token

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
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(token.LBRACKET, p.parseListLiteral)
	p.registerPrefix(token.LBRACE, p.parseDictLiteral)
	p.registerPrefix(token.LAMBDA, p.parseLambda)

	p.infixParseFns = make(map[token.TokenType]infixParseFn)
	p.registerInfix(token.PLUS, p.parseInfixExpression)
	p.registerInfix(token.MINUS, p.parseInfixExpression)
	p.registerInfix(token.SLASH, p.parseInfixExpression)
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
	p.registerInfix(token.LPAREN, p.parseCallExpression)
	p.registerInfix(token.LBRACKET, p.parseIndexExpression)
	p.registerInfix(token.DOT, p.parseIndexExpression)

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
	for p.peekToken.Type == token.NEWLINE {
		p.peekToken = p.l.NextToken()
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
		if p.curTokenIs(token.NEWLINE) {
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
	case token.DEF:
		return p.parseFunctionStatement()
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
	case token.TRY:
		return p.parseTryStatement()
	case token.RAISE:
		return p.parseRaiseStatement()
	case token.GLOBAL:
		return p.parseGlobalStatement()
	case token.NONLOCAL:
		return p.parseNonlocalStatement()
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
		p.peekTokenIs(token.MUL_EQ) || p.peekTokenIs(token.DIV_EQ) || p.peekTokenIs(token.MOD_EQ)
}

func (p *Parser) parseAssignStatement() *ast.AssignStatement {
	stmt := &ast.AssignStatement{Token: p.curToken}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.ASSIGN) {
		return nil
	}

	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)

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

	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	return stmt
}

func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	stmt := &ast.ReturnStatement{Token: p.curToken}
	p.nextToken()

	if !p.curTokenIs(token.NEWLINE) && !p.curTokenIs(token.EOF) {
		stmt.ReturnValue = p.parseExpression(LOWEST)
	}

	return stmt
}

func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {
	stmt := &ast.ExpressionStatement{Token: p.curToken}
	stmt.Expression = p.parseExpression(LOWEST)
	return stmt
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	leftExp := prefix()

	for !p.peekTokenIs(token.NEWLINE) && !p.peekTokenIs(token.EOF) && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}
		p.nextToken()
		leftExp = infix(leftExp)
	}

	return leftExp
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
	fstr.Parts, fstr.Expressions = p.parseFStringContent(p.curToken.Literal)
	return fstr
}

func (p *Parser) parseFStringContent(content string) ([]string, []ast.Expression) {
	parts := []string{}
	expressions := []ast.Expression{}
	current := ""
	i := 0

	for i < len(content) {
		if content[i] == '{' && i+1 < len(content) && content[i+1] != '{' {
			// Found expression start
			parts = append(parts, current)
			current = ""
			i++ // skip {

			// Extract expression until }
			exprStr := ""
			for i < len(content) && content[i] != '}' {
				exprStr += string(content[i])
				i++
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
	return parts, expressions
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

	firstExp := p.parseExpression(LOWEST)

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
			elements = append(elements, p.parseExpression(LOWEST))
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
	exp.Arguments, exp.Keywords = p.parseCallArguments()
	return exp
}

func (p *Parser) parseCallArguments() ([]ast.Expression, map[string]ast.Expression) {
	args := []ast.Expression{}
	keywords := make(map[string]ast.Expression)

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return args, keywords
	}

	p.nextToken()

	for {
		// Check for keyword argument: name=value
		if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.ASSIGN) {
			key := p.curToken.Literal
			if _, exists := keywords[key]; exists {
				msg := fmt.Sprintf("line %d: keyword argument repeated: %s", p.curToken.Line, key)
				p.errors = append(p.errors, msg)
				return nil, nil
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
				return nil, nil
			}

			expr := p.parseExpression(LOWEST)

			// Check for generator expression (for function arguments without parens)
			if p.peekTokenIs(token.FOR) {
				// This is a generator expression like: func(x for x in list)
				// parseGeneratorExpressionInCall expects to consume the end token (RPAREN)
				genExpr := p.parseGeneratorExpressionInCall(expr, token.RPAREN)
				if genExpr != nil {
					args = append(args, genExpr)
					return args, keywords
				}
			}

			args = append(args, expr)
		}

		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
			p.nextToken()
			continue
		}
		break
	}

	if !p.expectPeek(token.RPAREN) {
		return nil, nil
	}

	return args, keywords
}

func (p *Parser) parseExpressionList(end token.TokenType) []ast.Expression {
	list := []ast.Expression{}

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken()
	firstExpr := p.parseExpression(LOWEST)

	// Check if this is a generator expression (for function arguments without parens)
	if p.peekTokenIs(token.FOR) && end == token.RPAREN {
		// This is a generator expression like: func(x for x in list)
		genExpr := p.parseGeneratorExpressionInCall(firstExpr, end)
		if genExpr != nil {
			list = append(list, genExpr)
			return list
		}
	}

	list = append(list, firstExpr)

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		list = append(list, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
}

func (p *Parser) parseIfStatement() *ast.IfStatement {
	stmt := &ast.IfStatement{Token: p.curToken}

	p.nextToken()
	stmt.Condition = p.parseExpression(LOWEST)

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
		elifClause.Condition = p.parseExpression(LOWEST)

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
	stmt.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(token.COLON) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()

	return stmt
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	if !p.expectPeek(token.INDENT) {
		return nil
	}

	p.nextToken()

	for !p.curTokenIs(token.DEDENT) && !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.NEWLINE) {
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
	stmt.Function.Parameters, stmt.Function.DefaultValues = p.parseFunctionParameters()

	if !p.expectPeek(token.COLON) {
		return nil
	}

	stmt.Function.Body = p.parseBlockStatement()

	return stmt
}

func (p *Parser) parseFunctionParameters() ([]*ast.Identifier, map[string]ast.Expression) {
	identifiers := []*ast.Identifier{}
	defaults := make(map[string]ast.Expression)

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return identifiers, defaults
	}

	p.nextToken()

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
		return nil, nil
	}

	return identifiers, defaults
}

func (p *Parser) parseForStatement() *ast.ForStatement {
	stmt := &ast.ForStatement{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	stmt.Variable = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

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
	comp := &ast.ListComprehension{
		Token:      p.curToken,
		Expression: expr,
	}

	if !p.expectPeek(token.FOR) {
		return nil
	}

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	comp.Variable = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.IN) {
		return nil
	}

	p.nextToken()
	comp.Iterable = p.parseExpression(LOWEST)

	// Check for optional if condition
	if p.peekTokenIs(token.IF) {
		p.nextToken()
		p.nextToken()
		comp.Condition = p.parseExpression(LOWEST)
	}

	if !p.expectPeek(token.RBRACKET) {
		return nil
	}

	return comp
}

// parseGeneratorExpression parses generator expressions like (x for x in list)
// We treat them as list comprehensions for simplicity (eager evaluation)
func (p *Parser) parseGeneratorExpression(expr ast.Expression) ast.Expression {
	comp := &ast.ListComprehension{
		Token:      p.curToken,
		Expression: expr,
	}

	if !p.expectPeek(token.FOR) {
		return nil
	}

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	comp.Variable = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.IN) {
		return nil
	}

	p.nextToken()
	comp.Iterable = p.parseExpression(LOWEST)

	// Check for optional if condition
	if p.peekTokenIs(token.IF) {
		p.nextToken()
		p.nextToken()
		comp.Condition = p.parseExpression(LOWEST)
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return comp
}

// parseGeneratorExpressionInCall parses generator expressions in function calls
// like: func(x for x in list) - without surrounding parens
func (p *Parser) parseGeneratorExpressionInCall(expr ast.Expression, end token.TokenType) ast.Expression {
	comp := &ast.ListComprehension{
		Token:      p.curToken,
		Expression: expr,
	}

	if !p.expectPeek(token.FOR) {
		return nil
	}

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	comp.Variable = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.IN) {
		return nil
	}

	p.nextToken()
	comp.Iterable = p.parseExpression(LOWEST)

	// Check for optional if condition
	if p.peekTokenIs(token.IF) {
		p.nextToken()
		p.nextToken()
		comp.Condition = p.parseExpression(LOWEST)
	}

	if !p.expectPeek(end) {
		return nil
	}

	return comp
}

func (p *Parser) parseLambda() ast.Expression {
	lambda := &ast.Lambda{Token: p.curToken}

	// Parse parameters (optional)
	if !p.peekTokenIs(token.COLON) {
		lambda.Parameters, lambda.DefaultValues = p.parseLambdaParameters()
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

func (p *Parser) parseLambdaParameters() ([]*ast.Identifier, map[string]ast.Expression) {
	identifiers := []*ast.Identifier{}
	defaults := make(map[string]ast.Expression)

	p.nextToken()

	if p.curTokenIs(token.COLON) {
		return identifiers, defaults
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
		ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		identifiers = append(identifiers, ident)

		// Check for default value
		if p.peekTokenIs(token.ASSIGN) {
			p.nextToken() // consume =
			p.nextToken()
			defaults[ident.Value] = p.parseExpression(LOWEST)
		}
	}

	return identifiers, defaults
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

	if !p.curTokenIs(token.NEWLINE) && !p.curTokenIs(token.EOF) {
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

func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	if p.curTokenIs(token.DOT) {
		// Method call or member access: obj.method() or obj.member
		if !p.expectPeek(token.IDENT) {
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
			methodCall.Arguments, methodCall.Keywords = p.parseCallArguments()
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
