package ast

import "unsafe"

var (
	sizeOfProgram                 = int(unsafe.Sizeof(Program{}))
	sizeOfIdentifier              = int(unsafe.Sizeof(Identifier{}))
	sizeOfIntegerLiteral          = int(unsafe.Sizeof(IntegerLiteral{}))
	sizeOfFloatLiteral            = int(unsafe.Sizeof(FloatLiteral{}))
	sizeOfStringLiteral           = int(unsafe.Sizeof(StringLiteral{}))
	sizeOfBytesLiteral            = int(unsafe.Sizeof(BytesLiteral{}))
	sizeOfFStringLiteral          = int(unsafe.Sizeof(FStringLiteral{}))
	sizeOfBoolean                 = int(unsafe.Sizeof(Boolean{}))
	sizeOfNone                    = int(unsafe.Sizeof(None{}))
	sizeOfPrefixExpression        = int(unsafe.Sizeof(PrefixExpression{}))
	sizeOfInfixExpression         = int(unsafe.Sizeof(InfixExpression{}))
	sizeOfConditionalExpression   = int(unsafe.Sizeof(ConditionalExpression{}))
	sizeOfChainedComparison       = int(unsafe.Sizeof(ChainedComparison{}))
	sizeOfChainedLink             = int(unsafe.Sizeof(ChainedLink{}))
	sizeOfWalrusExpression        = int(unsafe.Sizeof(WalrusExpression{}))
	sizeOfAssignStatement         = int(unsafe.Sizeof(AssignStatement{}))
	sizeOfAugmentedAssign         = int(unsafe.Sizeof(AugmentedAssignStatement{}))
	sizeOfMultipleAssign          = int(unsafe.Sizeof(MultipleAssignStatement{}))
	sizeOfExpressionStatement     = int(unsafe.Sizeof(ExpressionStatement{}))
	sizeOfBlockStatement          = int(unsafe.Sizeof(BlockStatement{}))
	sizeOfElifClause              = int(unsafe.Sizeof(ElifClause{}))
	sizeOfIfStatement             = int(unsafe.Sizeof(IfStatement{}))
	sizeOfWhileStatement          = int(unsafe.Sizeof(WhileStatement{}))
	sizeOfFunctionLiteral         = int(unsafe.Sizeof(FunctionLiteral{}))
	sizeOfFunctionStatement       = int(unsafe.Sizeof(FunctionStatement{}))
	sizeOfClassStatement          = int(unsafe.Sizeof(ClassStatement{}))
	sizeOfCallExpression          = int(unsafe.Sizeof(CallExpression{}))
	sizeOfMethodCallExpression    = int(unsafe.Sizeof(MethodCallExpression{}))
	sizeOfReturnStatement         = int(unsafe.Sizeof(ReturnStatement{}))
	sizeOfBreakStatement          = int(unsafe.Sizeof(BreakStatement{}))
	sizeOfContinueStatement       = int(unsafe.Sizeof(ContinueStatement{}))
	sizeOfPassStatement           = int(unsafe.Sizeof(PassStatement{}))
	sizeOfDelStatement            = int(unsafe.Sizeof(DelStatement{}))
	sizeOfImportStatement         = int(unsafe.Sizeof(ImportStatement{}))
	sizeOfFromImportStatement     = int(unsafe.Sizeof(FromImportStatement{}))
	sizeOfForStatement            = int(unsafe.Sizeof(ForStatement{}))
	sizeOfListLiteral             = int(unsafe.Sizeof(ListLiteral{}))
	sizeOfDictLiteral             = int(unsafe.Sizeof(DictLiteral{}))
	sizeOfDictPairLiteral         = int(unsafe.Sizeof(DictPairLiteral{}))
	sizeOfSetLiteral              = int(unsafe.Sizeof(SetLiteral{}))
	sizeOfIndexExpression         = int(unsafe.Sizeof(IndexExpression{}))
	sizeOfSliceExpression         = int(unsafe.Sizeof(SliceExpression{}))
	sizeOfExceptClause            = int(unsafe.Sizeof(ExceptClause{}))
	sizeOfTryStatement            = int(unsafe.Sizeof(TryStatement{}))
	sizeOfRaiseStatement          = int(unsafe.Sizeof(RaiseStatement{}))
	sizeOfGlobalStatement         = int(unsafe.Sizeof(GlobalStatement{}))
	sizeOfNonlocalStatement       = int(unsafe.Sizeof(NonlocalStatement{}))
	sizeOfAssertStatement         = int(unsafe.Sizeof(AssertStatement{}))
	sizeOfComprehensionClause     = int(unsafe.Sizeof(ComprehensionClause{}))
	sizeOfListComprehension       = int(unsafe.Sizeof(ListComprehension{}))
	sizeOfDictComprehension       = int(unsafe.Sizeof(DictComprehension{}))
	sizeOfSetComprehension        = int(unsafe.Sizeof(SetComprehension{}))
	sizeOfLambda                  = int(unsafe.Sizeof(Lambda{}))
	sizeOfTupleLiteral            = int(unsafe.Sizeof(TupleLiteral{}))
	sizeOfWithStatement           = int(unsafe.Sizeof(WithStatement{}))
	sizeOfMatchStatement          = int(unsafe.Sizeof(MatchStatement{}))
	sizeOfCaseClause              = int(unsafe.Sizeof(CaseClause{}))
	sizeOfOrPattern               = int(unsafe.Sizeof(OrPattern{}))
	sizeOfStatementInterface      = int(unsafe.Sizeof((*Statement)(nil)))
	sizeOfExpressionInterface     = int(unsafe.Sizeof((*Expression)(nil)))
	sizeOfIdentifierPointer       = int(unsafe.Sizeof((*Identifier)(nil)))
	sizeOfString                  = int(unsafe.Sizeof(""))
	sizeOfDictPairSliceElem       = int(unsafe.Sizeof(DictPairLiteral{}))
	sizeOfComprehensionClauseElem = int(unsafe.Sizeof(ComprehensionClause{}))
)

type stringRef struct {
	ptr uintptr
	len int
}

// compiledNodeBytes is the retained cost of the evaluator's compiled form of
// one node: the closure tree the evaluator builds once per program and caches
// on it. Measured at 44 to 48 bytes per node across scripts of different
// shapes (October 2026), so one constant per node is accurate to a few
// percent.
const compiledNodeBytes = 48

// sizeClasses are the Go runtime's small-object allocation size classes. A
// node struct or string is allocated in the smallest class that fits it, so
// the retained size is the class size, not the struct size.
var sizeClasses = [...]int{8, 16, 24, 32, 48, 64, 80, 96, 112, 128, 144, 160, 176, 192, 208, 224, 240, 256, 288, 320, 352, 384, 416, 448, 480, 512, 576, 640, 704, 768, 896, 1024, 1152, 1280, 1408, 1536, 1792, 2048, 2304, 2688, 3072, 3200, 3456, 4096, 4864, 5376, 6144, 6528, 6784, 6912, 8192, 9472, 9728, 10240, 10880, 12288, 13568, 14336, 16384, 18432, 19072, 20480, 21760, 24576, 27264, 28672, 32768}

// smallAllocClass maps (n+7)/8 to the size class for n <= smallAllocMax, so
// the common case is one table lookup rather than a scan; the estimator runs
// once per node on every cache miss.
const smallAllocMax = 1024

var smallAllocClass [smallAllocMax/8 + 1]int

func init() {
	for n := 1; n <= smallAllocMax; n += 8 {
		for _, c := range sizeClasses {
			if n <= c {
				smallAllocClass[(n+7)/8] = c
				break
			}
		}
	}
}

// allocSize rounds n up to the size class the runtime would allocate it in.
// Larger objects are page-rounded.
func allocSize(n int) int {
	if n <= 0 {
		return 0
	}
	if n <= smallAllocMax {
		return smallAllocClass[(n+7)/8]
	}
	for _, c := range sizeClasses {
		if n <= c {
			return c
		}
	}
	const page = 8192
	return (n + page - 1) / page * page
}

// EstimateRetainedBytes approximates the retained heap size of a parsed AST
// together with the evaluator's compiled form of it, which is built once per
// program and retained alongside it. The source script is counted separately
// by callers, so strings that alias the source backing bytes are not charged
// again here.
func EstimateRetainedBytes(program *Program, source string) int {
	if program == nil {
		return 0
	}

	e := estimator{}
	if len(source) > 0 {
		e.sourcePtr = uintptr(unsafe.Pointer(unsafe.StringData(source)))
		e.sourceLen = uintptr(len(source))
	}
	if len(source) > 64 {
		e.seenNodes = make(map[uintptr]struct{}, 1024)
		e.seenStrings = make(map[stringRef]struct{}, 256)
	} else {
		e.skipDedup = true
	}
	e.walkProgram(program)
	return e.total
}

type estimator struct {
	total       int
	sourcePtr   uintptr
	sourceLen   uintptr
	skipDedup   bool
	seenNodes   map[uintptr]struct{}
	seenStrings map[stringRef]struct{}
}

func (e *estimator) mark(ptr uintptr, size int) bool {
	if ptr == 0 {
		return false
	}
	size = allocSize(size) + compiledNodeBytes
	if e.skipDedup {
		e.total += size
		return true
	}
	if _, ok := e.seenNodes[ptr]; ok {
		return false
	}
	e.seenNodes[ptr] = struct{}{}
	e.total += size
	return true
}

func (e *estimator) addString(s string) {
	if len(s) == 0 {
		return
	}
	ptr := uintptr(unsafe.Pointer(unsafe.StringData(s)))
	if ptr == 0 {
		return
	}
	if e.sourceLen > 0 && ptr >= e.sourcePtr && ptr < e.sourcePtr+e.sourceLen {
		return
	}
	if e.skipDedup {
		e.total += allocSize(len(s))
		return
	}
	ref := stringRef{ptr: ptr, len: len(s)}
	if _, ok := e.seenStrings[ref]; ok {
		return
	}
	e.seenStrings[ref] = struct{}{}
	e.total += allocSize(len(s))
}

func (e *estimator) addToken(tok any) {
	switch tok.(type) {
	case LineInfo:
		return
	}
}

func (e *estimator) addStatementSlice(items []Statement) {
	e.total += cap(items) * sizeOfStatementInterface
	for _, item := range items {
		e.walkStatement(item)
	}
}

func (e *estimator) addExpressionSlice(items []Expression) {
	e.total += cap(items) * sizeOfExpressionInterface
	for _, item := range items {
		e.walkExpression(item)
	}
}

func (e *estimator) addIdentifierSlice(items []*Identifier) {
	e.total += cap(items) * sizeOfIdentifierPointer
	for _, item := range items {
		e.walkIdentifier(item)
	}
}

func (e *estimator) addStringSlice(items []string) {
	e.total += cap(items) * sizeOfString
	for _, item := range items {
		e.addString(item)
	}
}

func (e *estimator) addKeywordMap(items map[string]Expression) {
	if len(items) == 0 {
		return
	}
	e.total += len(items) * (int(unsafe.Sizeof("")) + sizeOfExpressionInterface + 16)
	for key, value := range items {
		e.addString(key)
		e.walkExpression(value)
	}
}

func (e *estimator) addDefaultsMap(items map[string]Expression) {
	e.addKeywordMap(items)
}

func (e *estimator) walkProgram(program *Program) {
	if program == nil || !e.mark(uintptr(unsafe.Pointer(program)), sizeOfProgram) {
		return
	}
	e.addStatementSlice(program.Statements)
	if program.LocalSlots != nil {
		e.total += len(program.LocalSlots) * (int(unsafe.Sizeof("")) + 8)
	}
	e.addStringSlice(program.LocalSlotNames)
}

func (e *estimator) walkStatement(stmt Statement) {
	switch n := stmt.(type) {
	case *AssignStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfAssignStatement) {
			return
		}
		e.addToken(n.Token)
		e.walkExpression(n.Left)
		e.walkExpression(n.Value)
		e.walkStatement(n.Chained)
	case *AugmentedAssignStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfAugmentedAssign) {
			return
		}
		e.addToken(n.Token)
		e.walkIdentifier(n.Name)
		e.walkExpression(n.Left)
		e.walkExpression(n.Value)
	case *MultipleAssignStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfMultipleAssign) {
			return
		}
		e.addToken(n.Token)
		e.addIdentifierSlice(n.Names)
		e.walkExpression(n.Value)
	case *ExpressionStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfExpressionStatement) {
			return
		}
		e.addToken(n.Token)
		e.walkExpression(n.Expression)
	case *BlockStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfBlockStatement) {
			return
		}
		e.addToken(n.Token)
		e.addStatementSlice(n.Statements)
	case *IfStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfIfStatement) {
			return
		}
		e.addToken(n.Token)
		e.walkExpression(n.Condition)
		e.walkStatement(n.Consequence)
		e.total += cap(n.ElifClauses) * int(unsafe.Sizeof((*ElifClause)(nil)))
		for _, clause := range n.ElifClauses {
			e.walkElifClause(clause)
		}
		e.walkStatement(n.Alternative)
	case *WhileStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfWhileStatement) {
			return
		}
		e.addToken(n.Token)
		e.walkExpression(n.Condition)
		e.walkStatement(n.Body)
		e.walkStatement(n.Else)
	case *FunctionStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfFunctionStatement) {
			return
		}
		e.addToken(n.Token)
		e.walkIdentifier(n.Name)
		e.walkFunctionLiteral(n.Function)
		if n.overflow != nil {
			e.addExpressionSlice(n.overflow.Decorators)
		}
	case *ClassStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfClassStatement) {
			return
		}
		e.addToken(n.Token)
		e.walkIdentifier(n.Name)
		e.walkExpression(n.BaseClass)
		e.walkStatement(n.Body)
		if n.overflow != nil {
			e.addExpressionSlice(n.overflow.Decorators)
		}
	case *ReturnStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfReturnStatement) {
			return
		}
		e.addToken(n.Token)
		e.walkExpression(n.ReturnValue)
	case *BreakStatement:
		if n != nil && e.mark(uintptr(unsafe.Pointer(n)), sizeOfBreakStatement) {
			e.addToken(n.Token)
		}
	case *ContinueStatement:
		if n != nil && e.mark(uintptr(unsafe.Pointer(n)), sizeOfContinueStatement) {
			e.addToken(n.Token)
		}
	case *PassStatement:
		if n != nil && e.mark(uintptr(unsafe.Pointer(n)), sizeOfPassStatement) {
			e.addToken(n.Token)
		}
	case *DelStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfDelStatement) {
			return
		}
		e.addToken(n.Token)
		e.walkExpression(n.Target)
		for _, target := range n.ExtraTargets {
			e.walkExpression(target)
		}
	case *ImportStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfImportStatement) {
			return
		}
		e.addToken(n.Token)
		e.walkIdentifier(n.Name)
		if n.overflow != nil {
			e.walkIdentifier(n.overflow.Alias)
			e.addIdentifierSlice(n.overflow.AdditionalNames)
			e.addIdentifierSlice(n.overflow.AdditionalAliases)
		}
	case *FromImportStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfFromImportStatement) {
			return
		}
		e.addToken(n.Token)
		e.walkIdentifier(n.Module)
		e.addIdentifierSlice(n.Names)
		e.addIdentifierSlice(n.Aliases)
	case *ForStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfForStatement) {
			return
		}
		e.addToken(n.Token)
		e.addExpressionSlice(n.Variables)
		e.walkExpression(n.Iterable)
		e.walkStatement(n.Body)
		e.walkStatement(n.Else)
	case *TryStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfTryStatement) {
			return
		}
		e.addToken(n.Token)
		e.walkStatement(n.Body)
		e.total += cap(n.ExceptClauses) * int(unsafe.Sizeof((*ExceptClause)(nil)))
		for _, clause := range n.ExceptClauses {
			e.walkExceptClause(clause)
		}
		e.walkStatement(n.Else)
		e.walkStatement(n.Finally)
	case *RaiseStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfRaiseStatement) {
			return
		}
		e.addToken(n.Token)
		e.walkExpression(n.Message)
	case *GlobalStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfGlobalStatement) {
			return
		}
		e.addToken(n.Token)
		e.addIdentifierSlice(n.Names)
	case *NonlocalStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfNonlocalStatement) {
			return
		}
		e.addToken(n.Token)
		e.addIdentifierSlice(n.Names)
	case *AssertStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfAssertStatement) {
			return
		}
		e.addToken(n.Token)
		e.walkExpression(n.Condition)
		e.walkExpression(n.Message)
	case *WithStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfWithStatement) {
			return
		}
		e.addToken(n.Token)
		e.walkExpression(n.ContextExpr)
		e.walkIdentifier(n.Target)
		e.walkStatement(n.Body)
	case *MatchStatement:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfMatchStatement) {
			return
		}
		e.addToken(n.Token)
		e.walkExpression(n.Subject)
		e.total += cap(n.Cases) * int(unsafe.Sizeof((*CaseClause)(nil)))
		for _, clause := range n.Cases {
			e.walkCaseClause(clause)
		}
	}
}

func (e *estimator) walkExpression(expr Expression) {
	switch n := expr.(type) {
	case *Identifier:
		e.walkIdentifier(n)
	case *IntegerLiteral:
		if n != nil {
			e.mark(uintptr(unsafe.Pointer(n)), sizeOfIntegerLiteral)
		}
	case *FloatLiteral:
		if n != nil {
			e.mark(uintptr(unsafe.Pointer(n)), sizeOfFloatLiteral)
		}
	case *BytesLiteral:
		if n != nil {
			e.mark(uintptr(unsafe.Pointer(n)), sizeOfBytesLiteral)
		}
	case *StringLiteral:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfStringLiteral) {
			return
		}
		e.addString(n.Value)
	case *FStringLiteral:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfFStringLiteral) {
			return
		}
		e.addString(n.Value)
		e.addExpressionSlice(n.Expressions)
		e.addStringSlice(n.Parts)
		if n.overflow != nil {
			e.addStringSlice(n.overflow.FormatSpecs)
		}
	case *Boolean:
		if n != nil {
			e.mark(uintptr(unsafe.Pointer(n)), sizeOfBoolean)
		}
	case *None:
		if n != nil {
			e.mark(uintptr(unsafe.Pointer(n)), sizeOfNone)
		}
	case *PrefixExpression:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfPrefixExpression) {
			return
		}
		e.walkExpression(n.Right)
	case *InfixExpression:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfInfixExpression) {
			return
		}
		e.walkExpression(n.Left)
		e.walkExpression(n.Right)
	case *ConditionalExpression:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfConditionalExpression) {
			return
		}
		e.walkExpression(n.TrueExpr)
		e.walkExpression(n.Condition)
		e.walkExpression(n.FalseExpr)
	case *ChainedComparison:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfChainedComparison) {
			return
		}
		e.total += allocSize(cap(n.Links) * sizeOfChainedLink)
		e.walkExpression(n.First)
		for i := range n.Links {
			e.walkExpression(n.Links[i].Operand)
		}
	case *WalrusExpression:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfWalrusExpression) {
			return
		}
		e.walkExpression(n.Value)
	case *FunctionLiteral:
		e.walkFunctionLiteral(n)
	case *CallExpression:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfCallExpression) {
			return
		}
		e.walkExpression(n.Function)
		e.addExpressionSlice(n.Arguments)
		if n.HasOverflow() {
			e.addKeywordMap(n.GetKeywords())
			e.addExpressionSlice(n.GetArgsUnpack())
			e.walkExpression(n.GetKwargsUnpack())
		}
	case *MethodCallExpression:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfMethodCallExpression) {
			return
		}
		e.walkExpression(n.Receiver)
		e.walkIdentifier(n.Method)
		e.addExpressionSlice(n.Arguments)
		if n.HasOverflow() {
			e.addKeywordMap(n.GetKeywords())
			e.addExpressionSlice(n.GetArgsUnpack())
			e.walkExpression(n.GetKwargsUnpack())
		}
	case *ListLiteral:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfListLiteral) {
			return
		}
		e.addExpressionSlice(n.Elements)
	case *DictLiteral:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfDictLiteral) {
			return
		}
		e.total += cap(n.Pairs) * sizeOfDictPairSliceElem
		for _, pair := range n.Pairs {
			e.walkExpression(pair.Key)
			e.walkExpression(pair.Value)
		}
	case *SetLiteral:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfSetLiteral) {
			return
		}
		e.addExpressionSlice(n.Elements)
	case *IndexExpression:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfIndexExpression) {
			return
		}
		e.addToken(n.Token)
		e.walkExpression(n.Left)
		e.walkExpression(n.Index)
	case *SliceExpression:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfSliceExpression) {
			return
		}
		e.walkExpression(n.Left)
		e.walkExpression(n.Start)
		e.walkExpression(n.End)
		if n.overflow != nil {
			e.walkExpression(n.overflow.Step)
		}
	case *ListComprehension:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfListComprehension) {
			return
		}
		e.walkExpression(n.Expression)
		e.addExpressionSlice(n.Variables)
		e.walkExpression(n.Iterable)
		e.walkExpression(n.Condition)
		e.total += cap(n.AdditionalClauses) * sizeOfComprehensionClauseElem
		for _, clause := range n.AdditionalClauses {
			e.walkComprehensionClause(clause)
		}
	case *DictComprehension:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfDictComprehension) {
			return
		}
		e.walkExpression(n.Key)
		e.walkExpression(n.Value)
		e.addExpressionSlice(n.Variables)
		e.walkExpression(n.Iterable)
		e.walkExpression(n.Condition)
		e.total += cap(n.AdditionalClauses) * sizeOfComprehensionClauseElem
		for _, clause := range n.AdditionalClauses {
			e.walkComprehensionClause(clause)
		}
	case *SetComprehension:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfSetComprehension) {
			return
		}
		e.walkExpression(n.Expression)
		e.addExpressionSlice(n.Variables)
		e.walkExpression(n.Iterable)
		e.walkExpression(n.Condition)
		e.total += cap(n.AdditionalClauses) * sizeOfComprehensionClauseElem
		for _, clause := range n.AdditionalClauses {
			e.walkComprehensionClause(clause)
		}
	case *Lambda:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfLambda) {
			return
		}
		e.addIdentifierSlice(n.Parameters)
		e.addDefaultsMap(n.GetDefaultValues())
		e.walkIdentifier(n.GetVariadic())
		e.walkIdentifier(n.GetKwargs())
		e.walkExpression(n.Body)
	case *TupleLiteral:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfTupleLiteral) {
			return
		}
		e.addExpressionSlice(n.Elements)
	case *OrPattern:
		if n == nil || !e.mark(uintptr(unsafe.Pointer(n)), sizeOfOrPattern) {
			return
		}
		e.addExpressionSlice(n.Patterns)
	}
}

func (e *estimator) walkIdentifier(ident *Identifier) {
	if ident == nil || !e.mark(uintptr(unsafe.Pointer(ident)), sizeOfIdentifier) {
		return
	}
	e.addToken(ident.Token)
	e.addString(ident.Value())
}

func (e *estimator) walkFunctionLiteral(fn *FunctionLiteral) {
	if fn == nil || !e.mark(uintptr(unsafe.Pointer(fn)), sizeOfFunctionLiteral) {
		return
	}
	e.addIdentifierSlice(fn.Parameters)
	e.addDefaultsMap(fn.GetDefaultValues())
	e.walkIdentifier(fn.GetVariadic())
	e.walkIdentifier(fn.GetKwargs())
	e.walkStatement(fn.Body)
	if fn.LocalSlots != nil {
		e.total += len(fn.LocalSlots) * (int(unsafe.Sizeof("")) + 8)
	}
	e.addStringSlice(fn.LocalSlotNames)
}

func (e *estimator) walkElifClause(clause *ElifClause) {
	if clause == nil || !e.mark(uintptr(unsafe.Pointer(clause)), sizeOfElifClause) {
		return
	}
	e.addToken(clause.Token)
	e.walkExpression(clause.Condition)
	e.walkStatement(clause.Consequence)
}

func (e *estimator) walkExceptClause(clause *ExceptClause) {
	if clause == nil || !e.mark(uintptr(unsafe.Pointer(clause)), sizeOfExceptClause) {
		return
	}
	e.addToken(clause.Token)
	e.walkExpression(clause.ExceptType)
	e.walkIdentifier(clause.ExceptVar)
	e.walkStatement(clause.Body)
}

func (e *estimator) walkComprehensionClause(clause ComprehensionClause) {
	e.addExpressionSlice(clause.Variables)
	e.walkExpression(clause.Iterable)
	e.walkExpression(clause.Condition)
}

func (e *estimator) walkCaseClause(clause *CaseClause) {
	if clause == nil || !e.mark(uintptr(unsafe.Pointer(clause)), sizeOfCaseClause) {
		return
	}
	e.addToken(clause.Token)
	e.walkExpression(clause.Pattern)
	e.walkExpression(clause.Guard)
	e.walkStatement(clause.Body)
	e.walkIdentifier(clause.CaptureAs)
}
