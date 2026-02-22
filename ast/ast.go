package ast

import "github.com/paularlott/scriptling/token"

type Node interface {
	TokenLiteral() string
	Line() int
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

func (p *Program) Line() int {
	if len(p.Statements) > 0 {
		return p.Statements[0].Line()
	}
	return 0
}

type Identifier struct {
	Token token.Token
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) Line() int            { return i.Token.Line }

type IntegerLiteral struct {
	Token token.Token
	Value int64
}

func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntegerLiteral) Line() int            { return il.Token.Line }

type FloatLiteral struct {
	Token token.Token
	Value float64
}

func (fl *FloatLiteral) expressionNode()      {}
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FloatLiteral) Line() int            { return fl.Token.Line }

type StringLiteral struct {
	Token token.Token
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) Line() int            { return sl.Token.Line }

type FStringLiteral struct {
	Token       token.Token
	Value       string
	Expressions []Expression // expressions inside {}
	Parts       []string     // string parts between expressions
	FormatSpecs []string     // format specifiers for each expression (e.g., "2d" from {day:2d})
}

func (fsl *FStringLiteral) expressionNode()      {}
func (fsl *FStringLiteral) TokenLiteral() string { return fsl.Token.Literal }
func (fsl *FStringLiteral) Line() int            { return fsl.Token.Line }

type Boolean struct {
	Token token.Token
	Value bool
}

func (b *Boolean) expressionNode()      {}
func (b *Boolean) TokenLiteral() string { return b.Token.Literal }
func (b *Boolean) Line() int            { return b.Token.Line }

type None struct {
	Token token.Token
}

func (n *None) expressionNode()      {}
func (n *None) TokenLiteral() string { return n.Token.Literal }
func (n *None) Line() int            { return n.Token.Line }

type PrefixExpression struct {
	Token    token.Token
	Operator string
	Right    Expression
}

func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PrefixExpression) Line() int            { return pe.Token.Line }

type InfixExpression struct {
	Token    token.Token
	Left     Expression
	Operator string
	Right    Expression
}

func (ie *InfixExpression) expressionNode()      {}
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *InfixExpression) Line() int            { return ie.Token.Line }

type ConditionalExpression struct {
	Token     token.Token
	TrueExpr  Expression
	Condition Expression
	FalseExpr Expression
}

func (ce *ConditionalExpression) expressionNode()      {}
func (ce *ConditionalExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *ConditionalExpression) Line() int            { return ce.Token.Line }

type AssignStatement struct {
	Token token.Token
	Left  Expression
	Value Expression
}

func (as *AssignStatement) statementNode()       {}
func (as *AssignStatement) TokenLiteral() string { return as.Token.Literal }
func (as *AssignStatement) Line() int            { return as.Token.Line }

type AugmentedAssignStatement struct {
	Token    token.Token
	Name     *Identifier
	Operator string
	Value    Expression
}

func (aas *AugmentedAssignStatement) statementNode()       {}
func (aas *AugmentedAssignStatement) TokenLiteral() string { return aas.Token.Literal }
func (aas *AugmentedAssignStatement) Line() int            { return aas.Token.Line }

type MultipleAssignStatement struct {
	Token       token.Token
	Names       []*Identifier
	Value       Expression
	StarredIndex int // Index of starred variable (-1 if none)
}

func (mas *MultipleAssignStatement) statementNode()       {}
func (mas *MultipleAssignStatement) TokenLiteral() string { return mas.Token.Literal }
func (mas *MultipleAssignStatement) Line() int            { return mas.Token.Line }

type ExpressionStatement struct {
	Token      token.Token
	Expression Expression
}

func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExpressionStatement) Line() int            { return es.Token.Line }

type BlockStatement struct {
	Token      token.Token
	Statements []Statement
}

func (bs *BlockStatement) statementNode()       {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) Line() int            { return bs.Token.Line }

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
func (is *IfStatement) Line() int            { return is.Token.Line }

type WhileStatement struct {
	Token     token.Token
	Condition Expression
	Body      *BlockStatement
}

func (ws *WhileStatement) statementNode()       {}
func (ws *WhileStatement) TokenLiteral() string { return ws.Token.Literal }
func (ws *WhileStatement) Line() int            { return ws.Token.Line }

type FunctionLiteral struct {
	Token         token.Token
	Parameters    []*Identifier
	DefaultValues map[string]Expression // parameter name -> default value
	Variadic      *Identifier           // *args parameter (optional)
	Kwargs        *Identifier           // **kwargs parameter (optional)
	Body          *BlockStatement
}

func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FunctionLiteral) Line() int            { return fl.Token.Line }

type FunctionStatement struct {
	Token    token.Token
	Name     *Identifier
	Function *FunctionLiteral
}

func (fs *FunctionStatement) statementNode()       {}
func (fs *FunctionStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *FunctionStatement) Line() int            { return fs.Token.Line }

type ClassStatement struct {
	Token     token.Token
	Name      *Identifier
	BaseClass Expression // optional base class for inheritance (can be dotted like html.parser.HTMLParser)
	Body      *BlockStatement
}

func (cs *ClassStatement) statementNode()       {}
func (cs *ClassStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *ClassStatement) Line() int            { return cs.Token.Line }

type CallExpression struct {
	Token        token.Token
	Function     Expression
	Arguments    []Expression
	Keywords     map[string]Expression
	ArgsUnpack   []Expression // For *args unpacking (supports multiple)
	KwargsUnpack Expression   // For **kwargs unpacking
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) Line() int            { return ce.Token.Line }

type ReturnStatement struct {
	Token       token.Token
	ReturnValue Expression
}

func (rs *ReturnStatement) statementNode()       {}
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *ReturnStatement) Line() int            { return rs.Token.Line }

type BreakStatement struct {
	Token token.Token
}

func (bs *BreakStatement) statementNode()       {}
func (bs *BreakStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BreakStatement) Line() int            { return bs.Token.Line }

type ContinueStatement struct {
	Token token.Token
}

func (cs *ContinueStatement) statementNode()       {}
func (cs *ContinueStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *ContinueStatement) Line() int            { return cs.Token.Line }

type PassStatement struct {
	Token token.Token
}

func (ps *PassStatement) statementNode()       {}
func (ps *PassStatement) TokenLiteral() string { return ps.Token.Literal }
func (ps *PassStatement) Line() int            { return ps.Token.Line }

type ImportStatement struct {
	Token            token.Token
	Name             *Identifier   // The full dotted name stored as single identifier (e.g., "urllib.parse")
	Alias            *Identifier   // Optional alias for 'import X as Y'
	AdditionalNames  []*Identifier // For import lib1, lib2, lib3
	AdditionalAliases []*Identifier // Optional aliases for additional imports (for "import lib1 as alias1, lib2 as alias2")
}

func (is *ImportStatement) statementNode()       {}
func (is *ImportStatement) TokenLiteral() string { return is.Token.Literal }
func (is *ImportStatement) Line() int            { return is.Token.Line }

// FullName returns the complete import name (handles dotted imports like urllib.parse)
func (is *ImportStatement) FullName() string {
	return is.Name.Value
}

// FromImportStatement represents "from X import Y, Z" statements
type FromImportStatement struct {
	Token   token.Token   // The 'from' token
	Module  *Identifier   // The module name (e.g., "bs4" or "urllib.parse")
	Names   []*Identifier // The names to import (e.g., ["BeautifulSoup"])
	Aliases []*Identifier // Optional aliases (for "import X as Y"), nil if no alias
}

func (fis *FromImportStatement) statementNode()       {}
func (fis *FromImportStatement) TokenLiteral() string { return fis.Token.Literal }
func (fis *FromImportStatement) Line() int            { return fis.Token.Line }

type ForStatement struct {
	Token     token.Token
	Variables []Expression
	Iterable  Expression
	Body      *BlockStatement
}

func (fs *ForStatement) statementNode()       {}
func (fs *ForStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *ForStatement) Line() int            { return fs.Token.Line }

type ListLiteral struct {
	Token    token.Token
	Elements []Expression
}

func (ll *ListLiteral) expressionNode()      {}
func (ll *ListLiteral) TokenLiteral() string { return ll.Token.Literal }
func (ll *ListLiteral) Line() int            { return ll.Token.Line }

type DictLiteral struct {
	Token token.Token
	Pairs map[Expression]Expression
}

func (dl *DictLiteral) expressionNode()      {}
func (dl *DictLiteral) TokenLiteral() string { return dl.Token.Literal }
func (dl *DictLiteral) Line() int            { return dl.Token.Line }

type SetLiteral struct {
	Token    token.Token
	Elements []Expression
}

func (sl *SetLiteral) expressionNode()      {}
func (sl *SetLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *SetLiteral) Line() int            { return sl.Token.Line }

type IndexExpression struct {
	Token token.Token
	Left  Expression
	Index Expression
}

func (ie *IndexExpression) expressionNode()      {}
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IndexExpression) Line() int            { return ie.Token.Line }

type SliceExpression struct {
	Token token.Token
	Left  Expression
	Start Expression
	End   Expression
	Step  Expression
}

func (se *SliceExpression) expressionNode()      {}
func (se *SliceExpression) TokenLiteral() string { return se.Token.Literal }
func (se *SliceExpression) Line() int            { return se.Token.Line }

type ExceptClause struct {
	Token      token.Token
	ExceptType Expression  // exception type to catch
	ExceptVar  *Identifier // variable to bind exception to
	Body       *BlockStatement
}

type TryStatement struct {
	Token         token.Token
	Body          *BlockStatement
	ExceptClauses []*ExceptClause // multiple except blocks
	Finally       *BlockStatement
}

func (ts *TryStatement) statementNode()       {}
func (ts *TryStatement) TokenLiteral() string { return ts.Token.Literal }
func (ts *TryStatement) Line() int            { return ts.Token.Line }

type RaiseStatement struct {
	Token   token.Token
	Message Expression
}

func (rs *RaiseStatement) statementNode()       {}
func (rs *RaiseStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *RaiseStatement) Line() int            { return rs.Token.Line }

type GlobalStatement struct {
	Token token.Token
	Names []*Identifier
}

func (gs *GlobalStatement) statementNode()       {}
func (gs *GlobalStatement) TokenLiteral() string { return gs.Token.Literal }
func (gs *GlobalStatement) Line() int            { return gs.Token.Line }

type NonlocalStatement struct {
	Token token.Token
	Names []*Identifier
}

func (ns *NonlocalStatement) statementNode()       {}
func (ns *NonlocalStatement) TokenLiteral() string { return ns.Token.Literal }
func (ns *NonlocalStatement) Line() int            { return ns.Token.Line }

type AssertStatement struct {
	Token     token.Token
	Condition Expression
	Message   Expression // Optional
}

func (as *AssertStatement) statementNode()       {}
func (as *AssertStatement) TokenLiteral() string { return as.Token.Literal }
func (as *AssertStatement) Line() int            { return as.Token.Line }

type MethodCallExpression struct {
	Token        token.Token
	Object       Expression
	Method       *Identifier
	Arguments    []Expression
	Keywords     map[string]Expression
	ArgsUnpack   []Expression // For *args unpacking (supports multiple)
	KwargsUnpack Expression   // For **kwargs unpacking
}

func (mce *MethodCallExpression) expressionNode()      {}
func (mce *MethodCallExpression) TokenLiteral() string { return mce.Token.Literal }
func (mce *MethodCallExpression) Line() int            { return mce.Token.Line }

type ListComprehension struct {
	Token      token.Token
	Expression Expression
	Variables  []Expression // supports tuple unpacking like: for h, t in ...
	Iterable   Expression
	Condition  Expression // optional
}

func (lc *ListComprehension) expressionNode()      {}
func (lc *ListComprehension) TokenLiteral() string { return lc.Token.Literal }
func (lc *ListComprehension) Line() int            { return lc.Token.Line }

type Lambda struct {
	Token         token.Token
	Parameters    []*Identifier
	DefaultValues map[string]Expression
	Variadic      *Identifier // *args parameter (optional)
	Kwargs        *Identifier // **kwargs parameter (optional)
	Body          Expression  // single expression, not block
}

func (l *Lambda) expressionNode()      {}
func (l *Lambda) TokenLiteral() string { return l.Token.Literal }
func (l *Lambda) Line() int            { return l.Token.Line }

type TupleLiteral struct {
	Token    token.Token
	Elements []Expression
}

func (tl *TupleLiteral) expressionNode()      {}
func (tl *TupleLiteral) TokenLiteral() string { return tl.Token.Literal }
func (tl *TupleLiteral) Line() int            { return tl.Token.Line }

// MatchStatement represents a match statement with multiple case clauses
type MatchStatement struct {
	Token   token.Token // The 'match' token
	Subject Expression  // The expression to match against
	Cases   []*CaseClause
}

func (ms *MatchStatement) statementNode()       {}
func (ms *MatchStatement) TokenLiteral() string { return ms.Token.Literal }
func (ms *MatchStatement) Line() int            { return ms.Token.Line }

// CaseClause represents a single case in a match statement
type CaseClause struct {
	Token     token.Token // The 'case' token
	Pattern   Expression  // The pattern to match (can be literal, identifier for type, dict, or wildcard)
	Guard     Expression  // Optional guard condition (if clause)
	Body      *BlockStatement
	CaptureAs *Identifier // Optional capture variable for the matched value
}
