package ast

import "github.com/paularlott/scriptling/token"

type Node interface {
	TokenLiteral() string
}

type Statement interface {
	Node
	statementNode()
}

type Expression interface {
	Node
	expressionNode()
}

type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}

type Identifier struct {
	Token token.Token
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }

type IntegerLiteral struct {
	Token token.Token
	Value int64
}

func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }

type FloatLiteral struct {
	Token token.Token
	Value float64
}

func (fl *FloatLiteral) expressionNode()      {}
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Literal }

type StringLiteral struct {
	Token token.Token
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }

type FStringLiteral struct {
	Token token.Token
	Value string
	Expressions []Expression // expressions inside {}
	Parts []string // string parts between expressions
}

func (fsl *FStringLiteral) expressionNode()      {}
func (fsl *FStringLiteral) TokenLiteral() string { return fsl.Token.Literal }

type Boolean struct {
	Token token.Token
	Value bool
}

func (b *Boolean) expressionNode()      {}
func (b *Boolean) TokenLiteral() string { return b.Token.Literal }

type None struct {
	Token token.Token
}

func (n *None) expressionNode()      {}
func (n *None) TokenLiteral() string { return n.Token.Literal }

type PrefixExpression struct {
	Token    token.Token
	Operator string
	Right    Expression
}

func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }

type InfixExpression struct {
	Token    token.Token
	Left     Expression
	Operator string
	Right    Expression
}

func (ie *InfixExpression) expressionNode()      {}
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }

type AssignStatement struct {
	Token token.Token
	Name  *Identifier
	Value Expression
}

func (as *AssignStatement) statementNode()       {}
func (as *AssignStatement) TokenLiteral() string { return as.Token.Literal }

type AugmentedAssignStatement struct {
	Token    token.Token
	Name     *Identifier
	Operator string
	Value    Expression
}

func (aas *AugmentedAssignStatement) statementNode()       {}
func (aas *AugmentedAssignStatement) TokenLiteral() string { return aas.Token.Literal }

type MultipleAssignStatement struct {
	Token token.Token
	Names []*Identifier
	Value Expression
}

func (mas *MultipleAssignStatement) statementNode()       {}
func (mas *MultipleAssignStatement) TokenLiteral() string { return mas.Token.Literal }

type ExpressionStatement struct {
	Token      token.Token
	Expression Expression
}

func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }

type BlockStatement struct {
	Token      token.Token
	Statements []Statement
}

func (bs *BlockStatement) statementNode()       {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }

type ElifClause struct {
	Token       token.Token
	Condition   Expression
	Consequence *BlockStatement
}

type IfStatement struct {
	Token       token.Token
	Condition   Expression
	Consequence *BlockStatement
	ElifClauses []*ElifClause
	Alternative *BlockStatement
}

func (is *IfStatement) statementNode()       {}
func (is *IfStatement) TokenLiteral() string { return is.Token.Literal }

type WhileStatement struct {
	Token     token.Token
	Condition Expression
	Body      *BlockStatement
}

func (ws *WhileStatement) statementNode()       {}
func (ws *WhileStatement) TokenLiteral() string { return ws.Token.Literal }

type FunctionLiteral struct {
	Token         token.Token
	Parameters    []*Identifier
	DefaultValues map[string]Expression // parameter name -> default value
	Body          *BlockStatement
}

func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }

type FunctionStatement struct {
	Token    token.Token
	Name     *Identifier
	Function *FunctionLiteral
}

func (fs *FunctionStatement) statementNode()       {}
func (fs *FunctionStatement) TokenLiteral() string { return fs.Token.Literal }

type CallExpression struct {
	Token     token.Token
	Function  Expression
	Arguments []Expression
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }

type ReturnStatement struct {
	Token       token.Token
	ReturnValue Expression
}

func (rs *ReturnStatement) statementNode()       {}
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }

type BreakStatement struct {
	Token token.Token
}

func (bs *BreakStatement) statementNode()       {}
func (bs *BreakStatement) TokenLiteral() string { return bs.Token.Literal }

type ContinueStatement struct {
	Token token.Token
}

func (cs *ContinueStatement) statementNode()       {}
func (cs *ContinueStatement) TokenLiteral() string { return cs.Token.Literal }

type PassStatement struct {
	Token token.Token
}

func (ps *PassStatement) statementNode()       {}
func (ps *PassStatement) TokenLiteral() string { return ps.Token.Literal }

type ImportStatement struct {
	Token token.Token
	Name  *Identifier
}

func (is *ImportStatement) statementNode()       {}
func (is *ImportStatement) TokenLiteral() string { return is.Token.Literal }

type ForStatement struct {
	Token    token.Token
	Variable *Identifier
	Iterable Expression
	Body     *BlockStatement
}

func (fs *ForStatement) statementNode()       {}
func (fs *ForStatement) TokenLiteral() string { return fs.Token.Literal }

type ListLiteral struct {
	Token    token.Token
	Elements []Expression
}

func (ll *ListLiteral) expressionNode()      {}
func (ll *ListLiteral) TokenLiteral() string { return ll.Token.Literal }

type DictLiteral struct {
	Token token.Token
	Pairs map[Expression]Expression
}

func (dl *DictLiteral) expressionNode()      {}
func (dl *DictLiteral) TokenLiteral() string { return dl.Token.Literal }

type IndexExpression struct {
	Token token.Token
	Left  Expression
	Index Expression
}

func (ie *IndexExpression) expressionNode()      {}
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }

type SliceExpression struct {
	Token token.Token
	Left  Expression
	Start Expression
	End   Expression
}

func (se *SliceExpression) expressionNode()      {}
func (se *SliceExpression) TokenLiteral() string { return se.Token.Literal }

type TryStatement struct {
	Token       token.Token
	Body        *BlockStatement
	Except      *BlockStatement
	ExceptVar   *Identifier // for 'except Exception as e:'
	Finally     *BlockStatement
}

func (ts *TryStatement) statementNode()       {}
func (ts *TryStatement) TokenLiteral() string { return ts.Token.Literal }

type RaiseStatement struct {
	Token   token.Token
	Message Expression
}

func (rs *RaiseStatement) statementNode()       {}
func (rs *RaiseStatement) TokenLiteral() string { return rs.Token.Literal }

type GlobalStatement struct {
	Token token.Token
	Names []*Identifier
}

func (gs *GlobalStatement) statementNode()       {}
func (gs *GlobalStatement) TokenLiteral() string { return gs.Token.Literal }

type NonlocalStatement struct {
	Token token.Token
	Names []*Identifier
}

func (ns *NonlocalStatement) statementNode()       {}
func (ns *NonlocalStatement) TokenLiteral() string { return ns.Token.Literal }

type MethodCallExpression struct {
	Token     token.Token
	Object    Expression
	Method    *Identifier
	Arguments []Expression
}

func (mce *MethodCallExpression) expressionNode()      {}
func (mce *MethodCallExpression) TokenLiteral() string { return mce.Token.Literal }

type ListComprehension struct {
	Token      token.Token
	Expression Expression
	Variable   *Identifier
	Iterable   Expression
	Condition  Expression // optional
}

func (lc *ListComprehension) expressionNode()      {}
func (lc *ListComprehension) TokenLiteral() string { return lc.Token.Literal }

type Lambda struct {
	Token         token.Token
	Parameters    []*Identifier
	DefaultValues map[string]Expression
	Body          Expression // single expression, not block
}

func (l *Lambda) expressionNode()      {}
func (l *Lambda) TokenLiteral() string { return l.Token.Literal }

type TupleLiteral struct {
	Token    token.Token
	Elements []Expression
}

func (tl *TupleLiteral) expressionNode()      {}
func (tl *TupleLiteral) TokenLiteral() string { return tl.Token.Literal }


