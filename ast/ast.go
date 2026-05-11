package ast

import (
	"strings"
	"sync/atomic"

	"github.com/paularlott/scriptling/token"
)

type Op byte

const (
	OpNil Op = iota

	// Arithmetic group (1-12): OpAdd..OpDiv
	OpAdd
	OpSub
	OpMul
	OpFloorDiv
	OpMod
	OpPow
	OpBitAnd
	OpBitOr
	OpBitXor
	OpLShift
	OpRShift
	OpDiv

	// Comparison group (13-18): OpLt..OpNeq
	OpLt
	OpGt
	OpLte
	OpGte
	OpEq
	OpNeq

	// Short-circuit logical
	OpAnd
	OpOr

	// Membership/Identity
	OpIn
	OpNotIn
	OpIs
	OpIsNot

	// Prefix
	OpNot
	OpBitNot
	OpPos

	// Augmented assignment: base op + opAugOffset
	OpAddEq
	OpSubEq
	OpMulEq
	OpFloorDivEq
	OpModEq
	OpPowEq
	OpBitAndEq
	OpBitOrEq
	OpBitXorEq
	OpLShiftEq
	OpRShiftEq
	OpDivEq
)

const opAugOffset = OpAddEq - OpAdd

var opStrings = [...]string{
	OpNil:        "",
	OpAdd:        "+",
	OpSub:        "-",
	OpMul:        "*",
	OpFloorDiv:   "//",
	OpMod:        "%",
	OpPow:        "**",
	OpBitAnd:     "&",
	OpBitOr:      "|",
	OpBitXor:     "^",
	OpLShift:     "<<",
	OpRShift:     ">>",
	OpDiv:        "/",
	OpLt:         "<",
	OpGt:         ">",
	OpLte:        "<=",
	OpGte:        ">=",
	OpEq:         "==",
	OpNeq:        "!=",
	OpAnd:        "and",
	OpOr:         "or",
	OpIn:         "in",
	OpNotIn:      "not in",
	OpIs:         "is",
	OpIsNot:      "is not",
	OpNot:        "not",
	OpBitNot:     "~",
	OpPos:        "+",
	OpAddEq:      "+=",
	OpSubEq:      "-=",
	OpMulEq:      "*=",
	OpFloorDivEq: "//=",
	OpModEq:      "%=",
	OpPowEq:      "**=",
	OpBitAndEq:   "&=",
	OpBitOrEq:    "|=",
	OpBitXorEq:   "^=",
	OpLShiftEq:   "<<=",
	OpRShiftEq:   ">>=",
	OpDivEq:      "/=",
}

func (op Op) String() string {
	if int(op) < len(opStrings) {
		return opStrings[op]
	}
	return ""
}

var opLookup = map[string]Op{
	"+":    OpAdd,
	"-":    OpSub,
	"*":    OpMul,
	"/":    OpDiv,
	"//":   OpFloorDiv,
	"%":    OpMod,
	"**":   OpPow,
	"<":    OpLt,
	">":    OpGt,
	"<=":   OpLte,
	">=":   OpGte,
	"==":   OpEq,
	"!=":   OpNeq,
	"and":  OpAnd,
	"or":   OpOr,
	"in":       OpIn,
	"not in":   OpNotIn,
	"is":       OpIs,
	"is not":   OpIsNot,
	"&":    OpBitAnd,
	"|":    OpBitOr,
	"^":    OpBitXor,
	"<<":   OpLShift,
	">>":   OpRShift,
	"not":  OpNot,
	"~":    OpBitNot,
	"+=":   OpAddEq,
	"-=":   OpSubEq,
	"*=":   OpMulEq,
	"/=":   OpDivEq,
	"//=":  OpFloorDivEq,
	"%=":   OpModEq,
	"**=":  OpPowEq,
	"&=":   OpBitAndEq,
	"|=":   OpBitOrEq,
	"^=":   OpBitXorEq,
	"<<=":  OpLShiftEq,
	">>=":  OpRShiftEq,
}

func ParseOp(s string) Op {
	if op, ok := opLookup[s]; ok {
		return op
	}
	return OpNil
}

func (op Op) IsComparison() bool {
	return op >= OpLt && op <= OpNeq
}

func (op Op) IsArithmetic() bool {
	return op >= OpAdd && op <= OpDiv
}

func (op Op) BaseOp() Op {
	if op >= OpAddEq && op <= OpDivEq {
		return op - opAugOffset
	}
	return op
}

type LineInfo struct {
	Line int32
}

func NewLineInfo(tok token.Token) LineInfo {
	return LineInfo{Line: int32(tok.Line)}
}

// TokenInfo stores just the AST metadata we retain after parsing.
// The parser still uses full lexer tokens; cached AST nodes only need
// the original token literal and source line.
type TokenInfo struct {
	Literal string
	Line    int32
}

func NewTokenInfo(tok token.Token) TokenInfo {
	return TokenInfo{
		Literal: strings.Clone(tok.Literal),
		Line:    int32(tok.Line),
	}
}

type SymbolTable struct {
	names []string
	ids   map[string]uint32
}

const symbolTableMapThreshold = 8

func NewSymbolTable() *SymbolTable {
	return &SymbolTable{}
}

func (st *SymbolTable) Intern(name string) uint32 {
	if st == nil || name == "" {
		return 0
	}
	if st.ids != nil {
		if id, ok := st.ids[name]; ok {
			return id
		}
	} else {
		for idx, existing := range st.names {
			if existing == name {
				return uint32(idx + 1)
			}
		}
	}
	id := uint32(len(st.names) + 1)
	name = strings.Clone(name)
	st.names = append(st.names, name)
	if st.ids != nil {
		st.ids[name] = id
	} else if len(st.names) >= symbolTableMapThreshold {
		st.ids = make(map[string]uint32, len(st.names))
		for idx, existing := range st.names {
			st.ids[existing] = uint32(idx + 1)
		}
	}
	return id
}

func (st *SymbolTable) Resolve(id uint32) string {
	if st == nil || id == 0 || int(id) > len(st.names) {
		return ""
	}
	return st.names[id-1]
}

func (st *SymbolTable) Freeze() {
	if st != nil {
		st.ids = nil
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
	Symbols    *SymbolTable

	LocalSlots     map[string]int
	LocalSlotNames []string
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

func lineOfNode(node Node) int {
	if node == nil {
		return 0
	}
	return node.Line()
}

func lineOfExpr(expr Expression) int {
	if expr == nil {
		return 0
	}
	return expr.Line()
}

func lineOfStatement(stmt Statement) int {
	if stmt == nil {
		return 0
	}
	return stmt.Line()
}

func lineOfIdentifier(ident *Identifier) int {
	if ident == nil {
		return 0
	}
	return ident.Line()
}

func lineOfExprSlice(exprs []Expression) int {
	for _, expr := range exprs {
		if line := lineOfExpr(expr); line != 0 {
			return line
		}
	}
	return 0
}

func lineOfIdentifierSlice(idents []*Identifier) int {
	for _, ident := range idents {
		if line := lineOfIdentifier(ident); line != 0 {
			return line
		}
	}
	return 0
}

type Identifier struct {
	Token   LineInfo
	Symbols *SymbolTable
	Name    uint32
	// Slot cache for fast local variable access.
	// 0 = uncached (zero value), -1 = not a local slot, >0 = slot index + 1.
	SlotCache atomic.Int32
}

func NewIdentifier(tok token.Token, symbols *SymbolTable, value string) *Identifier {
	return &Identifier{Token: NewLineInfo(tok), Symbols: symbols, Name: symbols.Intern(value)}
}

func NewIdentifierWithLine(line LineInfo, symbols *SymbolTable, value string) *Identifier {
	return &Identifier{Token: line, Symbols: symbols, Name: symbols.Intern(value)}
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Value() }
func (i *Identifier) Line() int            { return int(i.Token.Line) }
func (i *Identifier) Value() string        { return i.Symbols.Resolve(i.Name) }

type IntegerLiteral struct {
	Value int64
}

func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return "" }
func (il *IntegerLiteral) Line() int            { return 0 }

type FloatLiteral struct {
	Value float64
}

func (fl *FloatLiteral) expressionNode()      {}
func (fl *FloatLiteral) TokenLiteral() string { return "" }
func (fl *FloatLiteral) Line() int            { return 0 }

type StringLiteral struct {
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Value }
func (sl *StringLiteral) Line() int            { return 0 }

type FStringOverflow struct {
	FormatSpecs []string
}

type FStringLiteral struct {
	Value       string
	Expressions []Expression
	Parts       []string
	overflow    *FStringOverflow
}

func (fsl *FStringLiteral) GetFormatSpecs() []string {
	if fsl.overflow == nil {
		return nil
	}
	return fsl.overflow.FormatSpecs
}

func (fsl *FStringLiteral) SetFormatSpecs(specs []string) {
	hasNonEmpty := false
	for _, s := range specs {
		if s != "" {
			hasNonEmpty = true
			break
		}
	}
	if !hasNonEmpty {
		return
	}
	fsl.overflow = &FStringOverflow{FormatSpecs: specs}
}

func (fsl *FStringLiteral) expressionNode()      {}
func (fsl *FStringLiteral) TokenLiteral() string { return fsl.Value }
func (fsl *FStringLiteral) Line() int            { return 0 }

var (
	BoolTrue  = &Boolean{Value: true}
	BoolFalse = &Boolean{Value: false}
	NoneLiteral = &None{}
)

type Boolean struct {
	Value bool
}

func (b *Boolean) expressionNode() {}
func (b *Boolean) TokenLiteral() string {
	if b.Value {
		return "True"
	}
	return "False"
}
func (b *Boolean) Line() int { return 0 }

type None struct {
}

func (n *None) expressionNode()      {}
func (n *None) TokenLiteral() string { return "None" }
func (n *None) Line() int            { return 0 }

type PrefixExpression struct {
	Operator Op
	Right    Expression
}

func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Operator.String() }
func (pe *PrefixExpression) Line() int            { return lineOfExpr(pe.Right) }

type InfixExpression struct {
	Left     Expression
	Operator Op
	Right    Expression
}

func (ie *InfixExpression) expressionNode()      {}
func (ie *InfixExpression) TokenLiteral() string { return ie.Operator.String() }
func (ie *InfixExpression) Line() int {
	if line := lineOfExpr(ie.Left); line != 0 {
		return line
	}
	return lineOfExpr(ie.Right)
}

type ConditionalExpression struct {
	TrueExpr  Expression
	Condition Expression
	FalseExpr Expression
}

func (ce *ConditionalExpression) expressionNode()      {}
func (ce *ConditionalExpression) TokenLiteral() string { return "if" }
func (ce *ConditionalExpression) Line() int {
	if line := lineOfExpr(ce.Condition); line != 0 {
		return line
	}
	if line := lineOfExpr(ce.TrueExpr); line != 0 {
		return line
	}
	return lineOfExpr(ce.FalseExpr)
}

type AssignStatement struct {
	Token   LineInfo
	Left    Expression
	Value   Expression
	Chained *AssignStatement // for chained assignment: a = b = 5
}

func (as *AssignStatement) statementNode() {}
func (as *AssignStatement) TokenLiteral() string {
	if as.Left != nil {
		return as.Left.TokenLiteral()
	}
	return "="
}
func (as *AssignStatement) Line() int { return int(as.Token.Line) }

type AugmentedAssignStatement struct {
	Token    LineInfo
	Name     *Identifier
	Operator Op
	Value    Expression
}

func (aas *AugmentedAssignStatement) statementNode()       {}
func (aas *AugmentedAssignStatement) TokenLiteral() string { return aas.Operator.String() }
func (aas *AugmentedAssignStatement) Line() int            { return int(aas.Token.Line) }

type MultipleAssignStatement struct {
	Token        LineInfo
	Names        []*Identifier
	Value        Expression
	StarredIndex int // Index of starred variable (-1 if none)
}

func (mas *MultipleAssignStatement) statementNode() {}
func (mas *MultipleAssignStatement) TokenLiteral() string {
	if len(mas.Names) > 0 && mas.Names[0] != nil {
		return mas.Names[0].TokenLiteral()
	}
	return "="
}
func (mas *MultipleAssignStatement) Line() int { return int(mas.Token.Line) }

type ExpressionStatement struct {
	Token      LineInfo
	Expression Expression
}

func (es *ExpressionStatement) statementNode() {}
func (es *ExpressionStatement) TokenLiteral() string {
	if es.Expression == nil {
		return ""
	}
	return es.Expression.TokenLiteral()
}
func (es *ExpressionStatement) Line() int { return int(es.Token.Line) }

type BlockStatement struct {
	Token      LineInfo
	Statements []Statement
}

func (bs *BlockStatement) statementNode() {}
func (bs *BlockStatement) TokenLiteral() string {
	if len(bs.Statements) > 0 {
		return bs.Statements[0].TokenLiteral()
	}
	return ""
}
func (bs *BlockStatement) Line() int { return int(bs.Token.Line) }

type ElifClause struct {
	Token       LineInfo
	Condition   Expression
	Consequence *BlockStatement
}

type IfStatement struct {
	Token       LineInfo
	Condition   Expression
	Consequence *BlockStatement
	ElifClauses []*ElifClause
	Alternative *BlockStatement
}

func (is *IfStatement) statementNode()       {}
func (is *IfStatement) TokenLiteral() string { return "if" }
func (is *IfStatement) Line() int            { return int(is.Token.Line) }

type WhileStatement struct {
	Token     LineInfo
	Condition Expression
	Body      *BlockStatement
	Else      *BlockStatement // optional else clause
}

func (ws *WhileStatement) statementNode()       {}
func (ws *WhileStatement) TokenLiteral() string { return "while" }
func (ws *WhileStatement) Line() int            { return int(ws.Token.Line) }

type FuncOverflow struct {
	DefaultValues map[string]Expression
	Variadic      *Identifier
	Kwargs        *Identifier
}

type FunctionLiteral struct {
	Parameters    []*Identifier
	overflow      *FuncOverflow
	Body          *BlockStatement
	HasNestedFunc bool

	LocalSlots       map[string]int
	LocalSlotNames   []string
	ParamSlotIndexes []int
}

func (fl *FunctionLiteral) GetDefaultValues() map[string]Expression {
	if fl.overflow == nil {
		return nil
	}
	return fl.overflow.DefaultValues
}

func (fl *FunctionLiteral) GetVariadic() *Identifier {
	if fl.overflow == nil {
		return nil
	}
	return fl.overflow.Variadic
}

func (fl *FunctionLiteral) GetKwargs() *Identifier {
	if fl.overflow == nil {
		return nil
	}
	return fl.overflow.Kwargs
}

func (fl *FunctionLiteral) SetFuncOverflow(dv map[string]Expression, variadic, kwargs *Identifier) {
	if dv == nil && variadic == nil && kwargs == nil {
		return
	}
	fl.overflow = &FuncOverflow{
		DefaultValues: dv,
		Variadic:      variadic,
		Kwargs:        kwargs,
	}
}

func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) TokenLiteral() string { return "def" }
func (fl *FunctionLiteral) Line() int {
	if line := lineOfIdentifierSlice(fl.Parameters); line != 0 {
		return line
	}
	if line := lineOfIdentifier(fl.GetVariadic()); line != 0 {
		return line
	}
	if line := lineOfIdentifier(fl.GetKwargs()); line != 0 {
		return line
	}
	return lineOfStatement(fl.Body)
}

type DecoratorsOverflow struct {
	Decorators []Expression
}

type FunctionStatement struct {
	Token    LineInfo
	Name     *Identifier
	Function *FunctionLiteral
	overflow *DecoratorsOverflow
}

func (fs *FunctionStatement) GetDecorators() []Expression {
	if fs.overflow == nil {
		return nil
	}
	return fs.overflow.Decorators
}

func (fs *FunctionStatement) SetDecorators(decorators []Expression) {
	if decorators == nil {
		return
	}
	fs.overflow = &DecoratorsOverflow{Decorators: decorators}
}

func (fs *FunctionStatement) statementNode()       {}
func (fs *FunctionStatement) TokenLiteral() string { return "def" }
func (fs *FunctionStatement) Line() int            { return int(fs.Token.Line) }

type ClassStatement struct {
	Token     LineInfo
	Name      *Identifier
	BaseClass Expression
	Body      *BlockStatement
	overflow  *DecoratorsOverflow
}

func (cs *ClassStatement) GetDecorators() []Expression {
	if cs.overflow == nil {
		return nil
	}
	return cs.overflow.Decorators
}

func (cs *ClassStatement) SetDecorators(decorators []Expression) {
	if decorators == nil {
		return
	}
	cs.overflow = &DecoratorsOverflow{Decorators: decorators}
}

func (cs *ClassStatement) statementNode()       {}
func (cs *ClassStatement) TokenLiteral() string { return "class" }
func (cs *ClassStatement) Line() int            { return int(cs.Token.Line) }

type CallOverflow struct {
	Keywords     map[string]Expression
	ArgsUnpack   []Expression
	KwargsUnpack Expression
}

type CallExpression struct {
	Function  Expression
	Arguments []Expression
	overflow  *CallOverflow
}

func (ce *CallExpression) GetKeywords() map[string]Expression {
	if ce.overflow == nil {
		return nil
	}
	return ce.overflow.Keywords
}

func (ce *CallExpression) GetArgsUnpack() []Expression {
	if ce.overflow == nil {
		return nil
	}
	return ce.overflow.ArgsUnpack
}

func (ce *CallExpression) GetKwargsUnpack() Expression {
	if ce.overflow == nil {
		return nil
	}
	return ce.overflow.KwargsUnpack
}

func (ce *CallExpression) SetOverflow(keywords map[string]Expression, argsUnpack []Expression, kwargsUnpack Expression) {
	if keywords == nil && argsUnpack == nil && kwargsUnpack == nil {
		return
	}
	ce.overflow = &CallOverflow{
		Keywords:     keywords,
		ArgsUnpack:   argsUnpack,
		KwargsUnpack: kwargsUnpack,
	}
}

func (ce *CallExpression) HasOverflow() bool {
	return ce.overflow != nil
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return "(" }
func (ce *CallExpression) Line() int {
	if line := lineOfExpr(ce.Function); line != 0 {
		return line
	}
	if line := lineOfExprSlice(ce.Arguments); line != 0 {
		return line
	}
	if ce.overflow != nil {
		for _, expr := range ce.overflow.Keywords {
			if line := lineOfExpr(expr); line != 0 {
				return line
			}
		}
		if line := lineOfExprSlice(ce.overflow.ArgsUnpack); line != 0 {
			return line
		}
		return lineOfExpr(ce.overflow.KwargsUnpack)
	}
	return 0
}

type MethodCallExpression struct {
	Receiver  Expression
	Method    *Identifier
	Arguments []Expression
	overflow  *CallOverflow
}

func (mce *MethodCallExpression) GetKeywords() map[string]Expression {
	if mce.overflow == nil {
		return nil
	}
	return mce.overflow.Keywords
}

func (mce *MethodCallExpression) GetArgsUnpack() []Expression {
	if mce.overflow == nil {
		return nil
	}
	return mce.overflow.ArgsUnpack
}

func (mce *MethodCallExpression) GetKwargsUnpack() Expression {
	if mce.overflow == nil {
		return nil
	}
	return mce.overflow.KwargsUnpack
}

func (mce *MethodCallExpression) SetOverflow(keywords map[string]Expression, argsUnpack []Expression, kwargsUnpack Expression) {
	if keywords == nil && argsUnpack == nil && kwargsUnpack == nil {
		return
	}
	mce.overflow = &CallOverflow{
		Keywords:     keywords,
		ArgsUnpack:   argsUnpack,
		KwargsUnpack: kwargsUnpack,
	}
}

func (mce *MethodCallExpression) HasOverflow() bool {
	return mce.overflow != nil
}

func (mce *MethodCallExpression) expressionNode()      {}
func (mce *MethodCallExpression) TokenLiteral() string { return "(" }
func (mce *MethodCallExpression) Line() int {
	if line := lineOfExpr(mce.Receiver); line != 0 {
		return line
	}
	if line := lineOfIdentifier(mce.Method); line != 0 {
		return line
	}
	if line := lineOfExprSlice(mce.Arguments); line != 0 {
		return line
	}
	if mce.overflow != nil {
		for _, expr := range mce.overflow.Keywords {
			if line := lineOfExpr(expr); line != 0 {
				return line
			}
		}
		if line := lineOfExprSlice(mce.overflow.ArgsUnpack); line != 0 {
			return line
		}
		return lineOfExpr(mce.overflow.KwargsUnpack)
	}
	return 0
}

type ReturnStatement struct {
	Token       LineInfo
	ReturnValue Expression
}

func (rs *ReturnStatement) statementNode()       {}
func (rs *ReturnStatement) TokenLiteral() string { return "return" }
func (rs *ReturnStatement) Line() int            { return int(rs.Token.Line) }

type BreakStatement struct {
	Token LineInfo
}

func (bs *BreakStatement) statementNode()       {}
func (bs *BreakStatement) TokenLiteral() string { return "break" }
func (bs *BreakStatement) Line() int            { return int(bs.Token.Line) }

type ContinueStatement struct {
	Token LineInfo
}

func (cs *ContinueStatement) statementNode()       {}
func (cs *ContinueStatement) TokenLiteral() string { return "continue" }
func (cs *ContinueStatement) Line() int            { return int(cs.Token.Line) }

type PassStatement struct {
	Token LineInfo
}

func (ps *PassStatement) statementNode()       {}
func (ps *PassStatement) TokenLiteral() string { return "pass" }
func (ps *PassStatement) Line() int            { return int(ps.Token.Line) }

type DelStatement struct {
	Token  LineInfo
	Target Expression
}

func (ds *DelStatement) statementNode()       {}
func (ds *DelStatement) TokenLiteral() string { return "del" }
func (ds *DelStatement) Line() int            { return int(ds.Token.Line) }

type ImportOverflow struct {
	Alias             *Identifier
	AdditionalNames   []*Identifier
	AdditionalAliases []*Identifier
}

type ImportStatement struct {
	Token   LineInfo
	Name    *Identifier
	overflow *ImportOverflow
}

func (is *ImportStatement) statementNode()       {}
func (is *ImportStatement) TokenLiteral() string { return "import" }
func (is *ImportStatement) Line() int            { return int(is.Token.Line) }

func (is *ImportStatement) GetAlias() *Identifier {
	if is.overflow == nil {
		return nil
	}
	return is.overflow.Alias
}

func (is *ImportStatement) GetAdditionalNames() []*Identifier {
	if is.overflow == nil {
		return nil
	}
	return is.overflow.AdditionalNames
}

func (is *ImportStatement) GetAdditionalAliases() []*Identifier {
	if is.overflow == nil {
		return nil
	}
	return is.overflow.AdditionalAliases
}

func (is *ImportStatement) SetImportOverflow(alias *Identifier, names, aliases []*Identifier) {
	if alias == nil && names == nil && aliases == nil {
		return
	}
	is.overflow = &ImportOverflow{
		Alias:             alias,
		AdditionalNames:   names,
		AdditionalAliases: aliases,
	}
}

func (is *ImportStatement) AppendAdditionalName(name *Identifier) {
	if is.overflow == nil {
		is.overflow = &ImportOverflow{}
	}
	if is.overflow.AdditionalNames == nil {
		is.overflow.AdditionalNames = make([]*Identifier, 0, 2)
	}
	is.overflow.AdditionalNames = append(is.overflow.AdditionalNames, name)
}

func (is *ImportStatement) AppendAdditionalAlias(alias *Identifier) {
	if is.overflow == nil {
		is.overflow = &ImportOverflow{}
	}
	if is.overflow.AdditionalAliases == nil {
		is.overflow.AdditionalAliases = make([]*Identifier, 0, 2)
	}
	is.overflow.AdditionalAliases = append(is.overflow.AdditionalAliases, alias)
}

// FullName returns the complete import name (handles dotted imports like urllib.parse)
func (is *ImportStatement) FullName() string {
	return is.Name.Value()
}

// FromImportStatement represents "from X import Y, Z" statements
// Also supports relative imports: "from . import X", "from .. import X", "from .module import X"
type FromImportStatement struct {
	Token         LineInfo      // The 'from' token
	Module        *Identifier   // The module name (e.g., "bs4" or "urllib.parse"), nil for "from . import X"
	Names         []*Identifier // The names to import (e.g., ["BeautifulSoup"])
	Aliases       []*Identifier // Optional aliases (for "import X as Y"), nil if no alias
	RelativeLevel int           // Number of leading dots for relative imports (0 = absolute, 1 = ".", 2 = "..", etc.)
}

func (fis *FromImportStatement) statementNode()       {}
func (fis *FromImportStatement) TokenLiteral() string { return "from" }
func (fis *FromImportStatement) Line() int            { return int(fis.Token.Line) }

type ForStatement struct {
	Token     LineInfo
	Variables []Expression
	Iterable  Expression
	Body      *BlockStatement
	Else      *BlockStatement // optional else clause
}

func (fs *ForStatement) statementNode()       {}
func (fs *ForStatement) TokenLiteral() string { return "for" }
func (fs *ForStatement) Line() int            { return int(fs.Token.Line) }

type ListLiteral struct {
	Elements []Expression
}

func (ll *ListLiteral) expressionNode()      {}
func (ll *ListLiteral) TokenLiteral() string { return "[" }
func (ll *ListLiteral) Line() int            { return lineOfExprSlice(ll.Elements) }

type DictLiteral struct {
	Pairs []DictPairLiteral
}

func (dl *DictLiteral) expressionNode()      {}
func (dl *DictLiteral) TokenLiteral() string { return "{" }
func (dl *DictLiteral) Line() int {
	for _, pair := range dl.Pairs {
		if line := lineOfExpr(pair.Key); line != 0 {
			return line
		}
		if line := lineOfExpr(pair.Value); line != 0 {
			return line
		}
	}
	return 0
}

type DictPairLiteral struct {
	Key   Expression
	Value Expression
}

type SetLiteral struct {
	Elements []Expression
}

func (sl *SetLiteral) expressionNode()      {}
func (sl *SetLiteral) TokenLiteral() string { return "{" }
func (sl *SetLiteral) Line() int            { return lineOfExprSlice(sl.Elements) }

type IndexExpression struct {
	Token       LineInfo
	Left        Expression
	Index       Expression
	IsDotAccess bool // true when desugared from dot notation (obj.attr)
}

func (ie *IndexExpression) expressionNode() {}
func (ie *IndexExpression) TokenLiteral() string {
	if ie.IsDotAccess {
		return "."
	}
	return "["
}
func (ie *IndexExpression) Line() int { return int(ie.Token.Line) }

type SliceOverflow struct {
	Step Expression
}

type SliceExpression struct {
	Left  Expression
	Start Expression
	End   Expression
	overflow *SliceOverflow
}

func (se *SliceExpression) GetStep() Expression {
	if se.overflow == nil {
		return nil
	}
	return se.overflow.Step
}

func (se *SliceExpression) SetStep(step Expression) {
	if step == nil {
		return
	}
	se.overflow = &SliceOverflow{Step: step}
}

func (se *SliceExpression) expressionNode()      {}
func (se *SliceExpression) TokenLiteral() string { return "[" }
func (se *SliceExpression) Line() int {
	if line := lineOfExpr(se.Left); line != 0 {
		return line
	}
	if line := lineOfExpr(se.Start); line != 0 {
		return line
	}
	if line := lineOfExpr(se.End); line != 0 {
		return line
	}
	return lineOfExpr(se.GetStep())
}

type ExceptClause struct {
	Token      LineInfo
	ExceptType Expression  // exception type to catch
	ExceptVar  *Identifier // variable to bind exception to
	Body       *BlockStatement
}

type TryStatement struct {
	Token         LineInfo
	Body          *BlockStatement
	ExceptClauses []*ExceptClause // multiple except blocks
	Else          *BlockStatement // runs only when no exception was raised
	Finally       *BlockStatement
}

func (ts *TryStatement) statementNode()       {}
func (ts *TryStatement) TokenLiteral() string { return "try" }
func (ts *TryStatement) Line() int            { return int(ts.Token.Line) }

type RaiseStatement struct {
	Token   LineInfo
	Message Expression
}

func (rs *RaiseStatement) statementNode()       {}
func (rs *RaiseStatement) TokenLiteral() string { return "raise" }
func (rs *RaiseStatement) Line() int            { return int(rs.Token.Line) }

type GlobalStatement struct {
	Token LineInfo
	Names []*Identifier
}

func (gs *GlobalStatement) statementNode()       {}
func (gs *GlobalStatement) TokenLiteral() string { return "global" }
func (gs *GlobalStatement) Line() int            { return int(gs.Token.Line) }

type NonlocalStatement struct {
	Token LineInfo
	Names []*Identifier
}

func (ns *NonlocalStatement) statementNode()       {}
func (ns *NonlocalStatement) TokenLiteral() string { return "nonlocal" }
func (ns *NonlocalStatement) Line() int            { return int(ns.Token.Line) }

type AssertStatement struct {
	Token     LineInfo
	Condition Expression
	Message   Expression // Optional
}

func (as *AssertStatement) statementNode()       {}
func (as *AssertStatement) TokenLiteral() string { return "assert" }
func (as *AssertStatement) Line() int            { return int(as.Token.Line) }

// ComprehensionClause represents an additional `for var in iterable [if cond]` clause
type ComprehensionClause struct {
	Variables []Expression
	Iterable  Expression
	Condition Expression // optional
}

type ListComprehension struct {
	Expression        Expression
	Variables         []Expression // supports tuple unpacking like: for h, t in ...
	Iterable          Expression
	Condition         Expression            // optional
	AdditionalClauses []ComprehensionClause // additional for clauses
}

func (lc *ListComprehension) expressionNode()      {}
func (lc *ListComprehension) TokenLiteral() string { return "[" }
func (lc *ListComprehension) Line() int {
	if line := lineOfExpr(lc.Expression); line != 0 {
		return line
	}
	if line := lineOfExprSlice(lc.Variables); line != 0 {
		return line
	}
	if line := lineOfExpr(lc.Iterable); line != 0 {
		return line
	}
	return lineOfExpr(lc.Condition)
}

type DictComprehension struct {
	Key               Expression
	Value             Expression
	Variables         []Expression
	Iterable          Expression
	Condition         Expression            // optional
	AdditionalClauses []ComprehensionClause // additional for clauses
}

func (dc *DictComprehension) expressionNode()      {}
func (dc *DictComprehension) TokenLiteral() string { return "{" }
func (dc *DictComprehension) Line() int {
	if line := lineOfExpr(dc.Key); line != 0 {
		return line
	}
	if line := lineOfExpr(dc.Value); line != 0 {
		return line
	}
	if line := lineOfExprSlice(dc.Variables); line != 0 {
		return line
	}
	if line := lineOfExpr(dc.Iterable); line != 0 {
		return line
	}
	return lineOfExpr(dc.Condition)
}

type SetComprehension struct {
	Expression        Expression
	Variables         []Expression
	Iterable          Expression
	Condition         Expression            // optional
	AdditionalClauses []ComprehensionClause // additional for clauses
}

func (sc *SetComprehension) expressionNode()      {}
func (sc *SetComprehension) TokenLiteral() string { return "{" }
func (sc *SetComprehension) Line() int {
	if line := lineOfExpr(sc.Expression); line != 0 {
		return line
	}
	if line := lineOfExprSlice(sc.Variables); line != 0 {
		return line
	}
	if line := lineOfExpr(sc.Iterable); line != 0 {
		return line
	}
	return lineOfExpr(sc.Condition)
}

type Lambda struct {
	Parameters []*Identifier
	overflow   *FuncOverflow
	Body       Expression

	LocalSlots       map[string]int
	LocalSlotNames   []string
	ParamSlotIndexes []int
}

func (l *Lambda) GetDefaultValues() map[string]Expression {
	if l.overflow == nil {
		return nil
	}
	return l.overflow.DefaultValues
}

func (l *Lambda) GetVariadic() *Identifier {
	if l.overflow == nil {
		return nil
	}
	return l.overflow.Variadic
}

func (l *Lambda) GetKwargs() *Identifier {
	if l.overflow == nil {
		return nil
	}
	return l.overflow.Kwargs
}

func (l *Lambda) SetFuncOverflow(dv map[string]Expression, variadic, kwargs *Identifier) {
	if dv == nil && variadic == nil && kwargs == nil {
		return
	}
	l.overflow = &FuncOverflow{
		DefaultValues: dv,
		Variadic:      variadic,
		Kwargs:        kwargs,
	}
}

func (l *Lambda) expressionNode()      {}
func (l *Lambda) TokenLiteral() string { return "lambda" }
func (l *Lambda) Line() int {
	if line := lineOfIdentifierSlice(l.Parameters); line != 0 {
		return line
	}
	if line := lineOfIdentifier(l.GetVariadic()); line != 0 {
		return line
	}
	if line := lineOfIdentifier(l.GetKwargs()); line != 0 {
		return line
	}
	return lineOfExpr(l.Body)
}

type TupleLiteral struct {
	Elements []Expression
}

func (tl *TupleLiteral) expressionNode()      {}
func (tl *TupleLiteral) TokenLiteral() string { return "(" }
func (tl *TupleLiteral) Line() int            { return lineOfExprSlice(tl.Elements) }

type WithStatement struct {
	Token       LineInfo
	ContextExpr Expression
	Target      *Identifier // optional: 'as' binding
	Body        *BlockStatement
}

func (ws *WithStatement) statementNode()       {}
func (ws *WithStatement) TokenLiteral() string { return "with" }
func (ws *WithStatement) Line() int            { return int(ws.Token.Line) }

// MatchStatement represents a match statement with multiple case clauses
type MatchStatement struct {
	Token   LineInfo   // The 'match' token
	Subject Expression // The expression to match against
	Cases   []*CaseClause
}

func (ms *MatchStatement) statementNode()       {}
func (ms *MatchStatement) TokenLiteral() string { return "match" }
func (ms *MatchStatement) Line() int            { return int(ms.Token.Line) }

// CaseClause represents a single case in a match statement
type CaseClause struct {
	Token     LineInfo   // The 'case' token
	Pattern   Expression // The pattern to match (can be literal, identifier for type, dict, or wildcard)
	Guard     Expression // Optional guard condition (if clause)
	Body      *BlockStatement
	CaptureAs *Identifier // Optional capture variable for the matched value
}

// OrPattern represents a pattern like `case 1 | 2 | 3:`
type OrPattern struct {
	Patterns []Expression
}

func (op *OrPattern) expressionNode()      {}
func (op *OrPattern) TokenLiteral() string { return "|" }
func (op *OrPattern) Line() int            { return lineOfExprSlice(op.Patterns) }
