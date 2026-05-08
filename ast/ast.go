package ast

import (
	"sync/atomic"

	"github.com/paularlott/scriptling/token"
)

// TokenInfo stores just the AST metadata we retain after parsing.
// The parser still uses full lexer tokens; cached AST nodes only need
// the original token literal and source line.
type TokenInfo struct {
	Literal string
	Line    int32
}

func NewTokenInfo(tok token.Token) TokenInfo {
	return TokenInfo{
		Literal: tok.Literal,
		Line:    int32(tok.Line),
	}
}

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
	Token TokenInfo
	Value string
	// Slot cache for fast local variable access.
	// 0 = uncached (zero value), -1 = not a local slot, >0 = slot index + 1.
	SlotCache atomic.Int32
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) Line() int            { return int(i.Token.Line) }

type IntegerLiteral struct {
	Token TokenInfo
	Value int64
}

func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntegerLiteral) Line() int            { return int(il.Token.Line) }

type FloatLiteral struct {
	Token TokenInfo
	Value float64
}

func (fl *FloatLiteral) expressionNode()      {}
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FloatLiteral) Line() int            { return int(fl.Token.Line) }

type StringLiteral struct {
	Token TokenInfo
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) Line() int            { return int(sl.Token.Line) }

type FStringLiteral struct {
	Token       TokenInfo
	Value       string
	Expressions []Expression // expressions inside {}
	Parts       []string     // string parts between expressions
	FormatSpecs []string     // format specifiers for each expression (e.g., "2d" from {day:2d})
}

func (fsl *FStringLiteral) expressionNode()      {}
func (fsl *FStringLiteral) TokenLiteral() string { return fsl.Token.Literal }
func (fsl *FStringLiteral) Line() int            { return int(fsl.Token.Line) }

type Boolean struct {
	Token TokenInfo
	Value bool
}

func (b *Boolean) expressionNode()      {}
func (b *Boolean) TokenLiteral() string { return b.Token.Literal }
func (b *Boolean) Line() int            { return int(b.Token.Line) }

type None struct {
	Token TokenInfo
}

func (n *None) expressionNode()      {}
func (n *None) TokenLiteral() string { return n.Token.Literal }
func (n *None) Line() int            { return int(n.Token.Line) }

type PrefixExpression struct {
	Token    TokenInfo
	Operator string
	Right    Expression
}

func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PrefixExpression) Line() int            { return int(pe.Token.Line) }

type InfixExpression struct {
	Token    TokenInfo
	Left     Expression
	Operator string
	Right    Expression
}

func (ie *InfixExpression) expressionNode()      {}
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *InfixExpression) Line() int            { return int(ie.Token.Line) }

type ConditionalExpression struct {
	Token     TokenInfo
	TrueExpr  Expression
	Condition Expression
	FalseExpr Expression
}

func (ce *ConditionalExpression) expressionNode()      {}
func (ce *ConditionalExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *ConditionalExpression) Line() int            { return int(ce.Token.Line) }

type AssignStatement struct {
	Token   TokenInfo
	Left    Expression
	Value   Expression
	Chained *AssignStatement // for chained assignment: a = b = 5
}

func (as *AssignStatement) statementNode()       {}
func (as *AssignStatement) TokenLiteral() string { return as.Token.Literal }
func (as *AssignStatement) Line() int            { return int(as.Token.Line) }

type AugmentedAssignStatement struct {
	Token    TokenInfo
	Name     *Identifier
	Operator string
	Value    Expression
}

func (aas *AugmentedAssignStatement) statementNode()       {}
func (aas *AugmentedAssignStatement) TokenLiteral() string { return aas.Token.Literal }
func (aas *AugmentedAssignStatement) Line() int            { return int(aas.Token.Line) }

type MultipleAssignStatement struct {
	Token        TokenInfo
	Names        []*Identifier
	Value        Expression
	StarredIndex int // Index of starred variable (-1 if none)
}

func (mas *MultipleAssignStatement) statementNode()       {}
func (mas *MultipleAssignStatement) TokenLiteral() string { return mas.Token.Literal }
func (mas *MultipleAssignStatement) Line() int            { return int(mas.Token.Line) }

type ExpressionStatement struct {
	Token      TokenInfo
	Expression Expression
}

func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExpressionStatement) Line() int            { return int(es.Token.Line) }

type BlockStatement struct {
	Token      TokenInfo
	Statements []Statement
}

func (bs *BlockStatement) statementNode()       {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) Line() int            { return int(bs.Token.Line) }

type ElifClause struct {
	Token       TokenInfo
	Condition   Expression
	Consequence *BlockStatement
}

type IfStatement struct {
	Token       TokenInfo
	Condition   Expression
	Consequence *BlockStatement
	ElifClauses []*ElifClause
	Alternative *BlockStatement
}

func (is *IfStatement) statementNode()       {}
func (is *IfStatement) TokenLiteral() string { return is.Token.Literal }
func (is *IfStatement) Line() int            { return int(is.Token.Line) }

type WhileStatement struct {
	Token     TokenInfo
	Condition Expression
	Body      *BlockStatement
	Else      *BlockStatement // optional else clause
}

func (ws *WhileStatement) statementNode()       {}
func (ws *WhileStatement) TokenLiteral() string { return ws.Token.Literal }
func (ws *WhileStatement) Line() int            { return int(ws.Token.Line) }

type FunctionLiteral struct {
	Token         TokenInfo
	Parameters    []*Identifier
	DefaultValues map[string]Expression // parameter name -> default value
	Variadic      *Identifier           // *args parameter (optional)
	Kwargs        *Identifier           // **kwargs parameter (optional)
	Body          *BlockStatement
	HasNestedFunc bool // set by parser: true if body contains any nested function/lambda/class
}

func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FunctionLiteral) Line() int            { return int(fl.Token.Line) }

type FunctionStatement struct {
	Token      TokenInfo
	Name       *Identifier
	Function   *FunctionLiteral
	Decorators []Expression // @decorator expressions, outermost first
}

func (fs *FunctionStatement) statementNode()       {}
func (fs *FunctionStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *FunctionStatement) Line() int            { return int(fs.Token.Line) }

type ClassStatement struct {
	Token      TokenInfo
	Name       *Identifier
	BaseClass  Expression // optional base class for inheritance (can be dotted like html.parser.HTMLParser)
	Body       *BlockStatement
	Decorators []Expression // @decorator expressions, outermost first
}

func (cs *ClassStatement) statementNode()       {}
func (cs *ClassStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *ClassStatement) Line() int            { return int(cs.Token.Line) }

type CallExpression struct {
	Token        TokenInfo
	Function     Expression
	Arguments    []Expression
	Keywords     map[string]Expression
	ArgsUnpack   []Expression // For *args unpacking (supports multiple)
	KwargsUnpack Expression   // For **kwargs unpacking
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) Line() int            { return int(ce.Token.Line) }

type ReturnStatement struct {
	Token       TokenInfo
	ReturnValue Expression
}

func (rs *ReturnStatement) statementNode()       {}
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *ReturnStatement) Line() int            { return int(rs.Token.Line) }

type BreakStatement struct {
	Token TokenInfo
}

func (bs *BreakStatement) statementNode()       {}
func (bs *BreakStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BreakStatement) Line() int            { return int(bs.Token.Line) }

type ContinueStatement struct {
	Token TokenInfo
}

func (cs *ContinueStatement) statementNode()       {}
func (cs *ContinueStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *ContinueStatement) Line() int            { return int(cs.Token.Line) }

type PassStatement struct {
	Token TokenInfo
}

func (ps *PassStatement) statementNode()       {}
func (ps *PassStatement) TokenLiteral() string { return ps.Token.Literal }
func (ps *PassStatement) Line() int            { return int(ps.Token.Line) }

type DelStatement struct {
	Token  TokenInfo
	Target Expression
}

func (ds *DelStatement) statementNode()       {}
func (ds *DelStatement) TokenLiteral() string { return ds.Token.Literal }
func (ds *DelStatement) Line() int            { return int(ds.Token.Line) }

type ImportStatement struct {
	Token             TokenInfo
	Name              *Identifier   // The full dotted name stored as single identifier (e.g., "urllib.parse")
	Alias             *Identifier   // Optional alias for 'import X as Y'
	AdditionalNames   []*Identifier // For import lib1, lib2, lib3
	AdditionalAliases []*Identifier // Optional aliases for additional imports (for "import lib1 as alias1, lib2 as alias2")
}

func (is *ImportStatement) statementNode()       {}
func (is *ImportStatement) TokenLiteral() string { return is.Token.Literal }
func (is *ImportStatement) Line() int            { return int(is.Token.Line) }

// FullName returns the complete import name (handles dotted imports like urllib.parse)
func (is *ImportStatement) FullName() string {
	return is.Name.Value
}

// FromImportStatement represents "from X import Y, Z" statements
// Also supports relative imports: "from . import X", "from .. import X", "from .module import X"
type FromImportStatement struct {
	Token         TokenInfo     // The 'from' token
	Module        *Identifier   // The module name (e.g., "bs4" or "urllib.parse"), nil for "from . import X"
	Names         []*Identifier // The names to import (e.g., ["BeautifulSoup"])
	Aliases       []*Identifier // Optional aliases (for "import X as Y"), nil if no alias
	RelativeLevel int           // Number of leading dots for relative imports (0 = absolute, 1 = ".", 2 = "..", etc.)
}

func (fis *FromImportStatement) statementNode()       {}
func (fis *FromImportStatement) TokenLiteral() string { return fis.Token.Literal }
func (fis *FromImportStatement) Line() int            { return int(fis.Token.Line) }

type ForStatement struct {
	Token     TokenInfo
	Variables []Expression
	Iterable  Expression
	Body      *BlockStatement
	Else      *BlockStatement // optional else clause
}

func (fs *ForStatement) statementNode()       {}
func (fs *ForStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *ForStatement) Line() int            { return int(fs.Token.Line) }

type ListLiteral struct {
	Token    TokenInfo
	Elements []Expression
}

func (ll *ListLiteral) expressionNode()      {}
func (ll *ListLiteral) TokenLiteral() string { return ll.Token.Literal }
func (ll *ListLiteral) Line() int            { return int(ll.Token.Line) }

type DictLiteral struct {
	Token TokenInfo
	Pairs []DictPairLiteral
}

func (dl *DictLiteral) expressionNode()      {}
func (dl *DictLiteral) TokenLiteral() string { return dl.Token.Literal }
func (dl *DictLiteral) Line() int            { return int(dl.Token.Line) }

type DictPairLiteral struct {
	Key   Expression
	Value Expression
}

type SetLiteral struct {
	Token    TokenInfo
	Elements []Expression
}

func (sl *SetLiteral) expressionNode()      {}
func (sl *SetLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *SetLiteral) Line() int            { return int(sl.Token.Line) }

type IndexExpression struct {
	Token       TokenInfo
	Left        Expression
	Index       Expression
	IsDotAccess bool // true when desugared from dot notation (obj.attr)
}

func (ie *IndexExpression) expressionNode()      {}
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IndexExpression) Line() int            { return int(ie.Token.Line) }

type SliceExpression struct {
	Token TokenInfo
	Left  Expression
	Start Expression
	End   Expression
	Step  Expression
}

func (se *SliceExpression) expressionNode()      {}
func (se *SliceExpression) TokenLiteral() string { return se.Token.Literal }
func (se *SliceExpression) Line() int            { return int(se.Token.Line) }

type ExceptClause struct {
	Token      TokenInfo
	ExceptType Expression  // exception type to catch
	ExceptVar  *Identifier // variable to bind exception to
	Body       *BlockStatement
}

type TryStatement struct {
	Token         TokenInfo
	Body          *BlockStatement
	ExceptClauses []*ExceptClause // multiple except blocks
	Else          *BlockStatement // runs only when no exception was raised
	Finally       *BlockStatement
}

func (ts *TryStatement) statementNode()       {}
func (ts *TryStatement) TokenLiteral() string { return ts.Token.Literal }
func (ts *TryStatement) Line() int            { return int(ts.Token.Line) }

type RaiseStatement struct {
	Token   TokenInfo
	Message Expression
}

func (rs *RaiseStatement) statementNode()       {}
func (rs *RaiseStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *RaiseStatement) Line() int            { return int(rs.Token.Line) }

type GlobalStatement struct {
	Token TokenInfo
	Names []*Identifier
}

func (gs *GlobalStatement) statementNode()       {}
func (gs *GlobalStatement) TokenLiteral() string { return gs.Token.Literal }
func (gs *GlobalStatement) Line() int            { return int(gs.Token.Line) }

type NonlocalStatement struct {
	Token TokenInfo
	Names []*Identifier
}

func (ns *NonlocalStatement) statementNode()       {}
func (ns *NonlocalStatement) TokenLiteral() string { return ns.Token.Literal }
func (ns *NonlocalStatement) Line() int            { return int(ns.Token.Line) }

type AssertStatement struct {
	Token     TokenInfo
	Condition Expression
	Message   Expression // Optional
}

func (as *AssertStatement) statementNode()       {}
func (as *AssertStatement) TokenLiteral() string { return as.Token.Literal }
func (as *AssertStatement) Line() int            { return int(as.Token.Line) }

type MethodCallExpression struct {
	Token        TokenInfo
	Object       Expression
	Method       *Identifier
	Arguments    []Expression
	Keywords     map[string]Expression
	ArgsUnpack   []Expression // For *args unpacking (supports multiple)
	KwargsUnpack Expression   // For **kwargs unpacking
}

func (mce *MethodCallExpression) expressionNode()      {}
func (mce *MethodCallExpression) TokenLiteral() string { return mce.Token.Literal }
func (mce *MethodCallExpression) Line() int            { return int(mce.Token.Line) }

// ComprehensionClause represents an additional `for var in iterable [if cond]` clause
type ComprehensionClause struct {
	Variables []Expression
	Iterable  Expression
	Condition Expression // optional
}

type ListComprehension struct {
	Token             TokenInfo
	Expression        Expression
	Variables         []Expression // supports tuple unpacking like: for h, t in ...
	Iterable          Expression
	Condition         Expression            // optional
	AdditionalClauses []ComprehensionClause // additional for clauses
}

func (lc *ListComprehension) expressionNode()      {}
func (lc *ListComprehension) TokenLiteral() string { return lc.Token.Literal }
func (lc *ListComprehension) Line() int            { return int(lc.Token.Line) }

type DictComprehension struct {
	Token             TokenInfo
	Key               Expression
	Value             Expression
	Variables         []Expression
	Iterable          Expression
	Condition         Expression            // optional
	AdditionalClauses []ComprehensionClause // additional for clauses
}

func (dc *DictComprehension) expressionNode()      {}
func (dc *DictComprehension) TokenLiteral() string { return dc.Token.Literal }
func (dc *DictComprehension) Line() int            { return int(dc.Token.Line) }

type SetComprehension struct {
	Token             TokenInfo
	Expression        Expression
	Variables         []Expression
	Iterable          Expression
	Condition         Expression            // optional
	AdditionalClauses []ComprehensionClause // additional for clauses
}

func (sc *SetComprehension) expressionNode()      {}
func (sc *SetComprehension) TokenLiteral() string { return sc.Token.Literal }
func (sc *SetComprehension) Line() int            { return int(sc.Token.Line) }

type Lambda struct {
	Token         TokenInfo
	Parameters    []*Identifier
	DefaultValues map[string]Expression
	Variadic      *Identifier // *args parameter (optional)
	Kwargs        *Identifier // **kwargs parameter (optional)
	Body          Expression  // single expression, not block
}

func (l *Lambda) expressionNode()      {}
func (l *Lambda) TokenLiteral() string { return l.Token.Literal }
func (l *Lambda) Line() int            { return int(l.Token.Line) }

type TupleLiteral struct {
	Token    TokenInfo
	Elements []Expression
}

func (tl *TupleLiteral) expressionNode()      {}
func (tl *TupleLiteral) TokenLiteral() string { return tl.Token.Literal }
func (tl *TupleLiteral) Line() int            { return int(tl.Token.Line) }

type WithStatement struct {
	Token       TokenInfo
	ContextExpr Expression
	Target      *Identifier // optional: 'as' binding
	Body        *BlockStatement
}

func (ws *WithStatement) statementNode()       {}
func (ws *WithStatement) TokenLiteral() string { return ws.Token.Literal }
func (ws *WithStatement) Line() int            { return int(ws.Token.Line) }

// MatchStatement represents a match statement with multiple case clauses
type MatchStatement struct {
	Token   TokenInfo  // The 'match' token
	Subject Expression // The expression to match against
	Cases   []*CaseClause
}

func (ms *MatchStatement) statementNode()       {}
func (ms *MatchStatement) TokenLiteral() string { return ms.Token.Literal }
func (ms *MatchStatement) Line() int            { return int(ms.Token.Line) }

// CaseClause represents a single case in a match statement
type CaseClause struct {
	Token     TokenInfo  // The 'case' token
	Pattern   Expression // The pattern to match (can be literal, identifier for type, dict, or wildcard)
	Guard     Expression // Optional guard condition (if clause)
	Body      *BlockStatement
	CaptureAs *Identifier // Optional capture variable for the matched value
}

// OrPattern represents a pattern like `case 1 | 2 | 3:`
type OrPattern struct {
	Token    TokenInfo
	Patterns []Expression
}

func (op *OrPattern) expressionNode()      {}
func (op *OrPattern) TokenLiteral() string { return op.Token.Literal }
func (op *OrPattern) Line() int            { return int(op.Token.Line) }
