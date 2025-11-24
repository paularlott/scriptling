package parser

import (
	"fmt"
	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/token"
	"strconv"
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
	PREFIX
	CALL
)

var precedences = map[token.TokenType]int{
	token.OR:       OR,
	token.AND:      AND,
	token.EQ:       EQUALS,
	token.NOT_EQ:   EQUALS,
	token.LT:       LESSGREATER,
	token.GT:       LESSGREATER,
	token.LTE:      LESSGREATER,
	token.GTE:      LESSGREATER,
	token.PLUS:     SUM,
	token.MINUS:    SUM,
	token.SLASH:    PRODUCT,
	token.ASTERISK: PRODUCT,
	token.PERCENT:  PRODUCT,
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
	p.registerPrefix(token.TRUE, p.parseBoolean)
	p.registerPrefix(token.FALSE, p.parseBoolean)
	p.registerPrefix(token.NONE, p.parseNone)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)
	p.registerPrefix(token.NOT, p.parsePrefixExpression)
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(token.LBRACKET, p.parseListLiteral)
	p.registerPrefix(token.LBRACE, p.parseDictLiteral)

	p.infixParseFns = make(map[token.TokenType]infixParseFn)
	p.registerInfix(token.PLUS, p.parseInfixExpression)
	p.registerInfix(token.MINUS, p.parseInfixExpression)
	p.registerInfix(token.SLASH, p.parseInfixExpression)
	p.registerInfix(token.ASTERISK, p.parseInfixExpression)
	p.registerInfix(token.PERCENT, p.parseInfixExpression)
	p.registerInfix(token.EQ, p.parseInfixExpression)
	p.registerInfix(token.NOT_EQ, p.parseInfixExpression)
	p.registerInfix(token.LT, p.parseInfixExpression)
	p.registerInfix(token.GT, p.parseInfixExpression)
	p.registerInfix(token.LTE, p.parseInfixExpression)
	p.registerInfix(token.GTE, p.parseInfixExpression)
	p.registerInfix(token.AND, p.parseInfixExpression)
	p.registerInfix(token.OR, p.parseInfixExpression)
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
	case token.IDENT:
		if p.peekTokenIs(token.ASSIGN) {
			return p.parseAssignStatement()
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

func (p *Parser) parseAugmentedAssignStatement() *ast.AugmentedAssignStatement {
	stmt := &ast.AugmentedAssignStatement{Token: p.curToken}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	p.nextToken()
	stmt.Operator = p.curToken.Literal

	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)

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
	p.nextToken()
	expression.Right = p.parseExpression(precedence)
	return expression
}

func (p *Parser) parseGroupedExpression() ast.Expression {
	p.nextToken()
	exp := p.parseExpression(LOWEST)
	if !p.expectPeek(token.RPAREN) {
		return nil
	}
	return exp
}

func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	exp := &ast.CallExpression{Token: p.curToken, Function: function}
	exp.Arguments = p.parseExpressionList(token.RPAREN)
	return exp
}

func (p *Parser) parseExpressionList(end token.TokenType) []ast.Expression {
	list := []ast.Expression{}

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken()
	list = append(list, p.parseExpression(LOWEST))

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
	stmt.Function.Parameters = p.parseFunctionParameters()

	if !p.expectPeek(token.COLON) {
		return nil
	}

	stmt.Function.Body = p.parseBlockStatement()

	return stmt
}

func (p *Parser) parseFunctionParameters() []*ast.Identifier {
	identifiers := []*ast.Identifier{}

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return identifiers
	}

	p.nextToken()

	ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	identifiers = append(identifiers, ident)

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		identifiers = append(identifiers, ident)
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return identifiers
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
	list.Elements = p.parseExpressionList(token.RBRACKET)
	return list
}

func (p *Parser) parseDictLiteral() ast.Expression {
	dict := &ast.DictLiteral{Token: p.curToken}
	dict.Pairs = make(map[ast.Expression]ast.Expression)

	if p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		return dict
	}

	p.nextToken()

	for {
		key := p.parseExpression(LOWEST)

		if !p.expectPeek(token.COLON) {
			return nil
		}

		p.nextToken()
		value := p.parseExpression(LOWEST)

		dict.Pairs[key] = value

		if !p.peekTokenIs(token.COMMA) {
			break
		}
		p.nextToken()
		p.nextToken()
	}

	if !p.expectPeek(token.RBRACE) {
		return nil
	}

	return dict
}

func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	if p.curTokenIs(token.DOT) {
		// Member access: obj.member
		exp := &ast.IndexExpression{Token: p.curToken, Left: left}
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		exp.Index = &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
		return exp
	}

	// Bracket access: obj[index] or obj[start:end]
	tok := p.curToken
	p.nextToken()

	// Check for slice notation
	if p.curTokenIs(token.COLON) {
		// Slice with no start: [:end]
		slice := &ast.SliceExpression{Token: tok, Left: left, Start: nil}
		if !p.peekTokenIs(token.RBRACKET) {
			p.nextToken()
			slice.End = p.parseExpression(LOWEST)
		}
		if !p.expectPeek(token.RBRACKET) {
			return nil
		}
		return slice
	}

	start := p.parseExpression(LOWEST)

	if p.peekTokenIs(token.COLON) {
		// Slice notation: [start:end]
		p.nextToken() // consume colon
		slice := &ast.SliceExpression{Token: tok, Left: left, Start: start}
		if !p.peekTokenIs(token.RBRACKET) {
			p.nextToken()
			slice.End = p.parseExpression(LOWEST)
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
