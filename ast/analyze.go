package ast

import (
	"strings"
)

func AnalyzeTopLevelLocals(program *Program) {
	names := make([]string, 0, 8)
	seen := make(map[string]struct{}, 8)

	addName := func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	globals := make(map[string]bool)
	nonlocals := make(map[string]bool)

	for _, stmt := range program.Statements {
		if stmt == nil {
			continue
		}
		switch s := stmt.(type) {
		case *GlobalStatement:
			for _, name := range s.Names {
				globals[name.Value()] = true
			}
		case *NonlocalStatement:
			for _, name := range s.Names {
				nonlocals[name.Value()] = true
			}
		}
	}

	for _, stmt := range program.Statements {
		if stmt == nil {
			continue
		}
		collectAssignedNamesFromStatement(stmt, globals, nonlocals, addName)
	}

	if len(names) == 0 {
		return
	}

	slots := make(map[string]int, len(names))
	nameIDs := make([]uint32, len(names))
	for idx, name := range names {
		slots[name] = idx
		nameIDs[idx] = program.Symbols.Intern(name)
	}
	program.LocalSlots = slots
	program.LocalSlotNames = names
	program.LocalSlotNameIDs = nameIDs
}

func AnalyzeFunctionLocals(fn *FunctionLiteral) {
	names := make([]string, 0, len(fn.Parameters)+4)
	seen := make(map[string]struct{}, len(fn.Parameters)+4)

	addName := func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	for _, param := range fn.Parameters {
		addName(param.Value())
	}
	if fn.GetVariadic() != nil {
		addName(fn.GetVariadic().Value())
	}
	if fn.GetKwargs() != nil {
		addName(fn.GetKwargs().Value())
	}

	globals, nonlocals := collectScopeDirectives(fn.Body)
	collectAssignedNamesFromBlock(fn.Body, globals, nonlocals, addName)

	slots := make(map[string]int, len(names))
	nameIDs := make([]uint32, len(names))
	var symbols *SymbolTable
	if len(fn.Parameters) > 0 {
		symbols = fn.Parameters[0].Symbols
	}
	for idx, name := range names {
		slots[name] = idx
		if symbols != nil {
			nameIDs[idx] = symbols.Intern(name)
		}
	}
	fn.LocalSlots = slots
	fn.LocalSlotNames = names
	if symbols != nil {
		fn.LocalSlotNameIDs = nameIDs
	}
	fn.ParamSlotIndexes = computeParamSlotIndexes(fn.Parameters, slots)
}

func AnalyzeLambdaLocals(lambda *Lambda) {
	names := make([]string, 0, len(lambda.Parameters)+2)
	for _, param := range lambda.Parameters {
		names = append(names, param.Value())
	}
	if lambda.GetVariadic() != nil {
		names = append(names, lambda.GetVariadic().Value())
	}
	if lambda.GetKwargs() != nil {
		names = append(names, lambda.GetKwargs().Value())
	}
	if len(names) == 0 {
		return
	}
	slots := make(map[string]int, len(names))
	uniq := names[:0]
	uniqIDs := make([]uint32, 0, len(names))
	var symbols *SymbolTable
	if len(lambda.Parameters) > 0 {
		symbols = lambda.Parameters[0].Symbols
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		if _, ok := slots[name]; ok {
			continue
		}
		slots[name] = len(uniq)
		var id uint32
		if symbols != nil {
			id = symbols.Intern(name)
		}
		uniq = append(uniq, name)
		uniqIDs = append(uniqIDs, id)
	}
	lambda.LocalSlots = slots
	lambda.LocalSlotNames = uniq
	if symbols != nil {
		lambda.LocalSlotNameIDs = uniqIDs
	}
	lambda.ParamSlotIndexes = computeParamSlotIndexes(lambda.Parameters, slots)
}

func computeParamSlotIndexes(parameters []*Identifier, slotIndex map[string]int) []int {
	if len(parameters) == 0 || len(slotIndex) == 0 {
		return nil
	}
	indexes := make([]int, len(parameters))
	for i, param := range parameters {
		idx, ok := slotIndex[param.Value()]
		if !ok {
			return nil
		}
		indexes[i] = idx
	}
	return indexes
}

func collectScopeDirectives(block *BlockStatement) (map[string]bool, map[string]bool) {
	globals := make(map[string]bool)
	nonlocals := make(map[string]bool)
	if block == nil {
		return globals, nonlocals
	}
	for _, stmt := range block.Statements {
		switch s := stmt.(type) {
		case *GlobalStatement:
			for _, name := range s.Names {
				globals[name.Value()] = true
			}
		case *NonlocalStatement:
			for _, name := range s.Names {
				nonlocals[name.Value()] = true
			}
		}
	}
	return globals, nonlocals
}

func collectAssignedNamesFromBlock(block *BlockStatement, globals map[string]bool, nonlocals map[string]bool, addName func(string)) {
	if block == nil {
		return
	}
	for _, stmt := range block.Statements {
		collectAssignedNamesFromStatement(stmt, globals, nonlocals, addName)
	}
}

func collectAssignedNamesFromStatement(stmt Statement, globals map[string]bool, nonlocals map[string]bool, addName func(string)) {
	if stmt == nil {
		return
	}

	addLocal := func(name string) {
		if name == "" || globals[name] || nonlocals[name] {
			return
		}
		addName(name)
	}

	switch s := stmt.(type) {
	case *AssignStatement:
		collectAssignedNamesFromExpression(s.Left, addLocal)
		if s.Chained != nil {
			collectAssignedNamesFromStatement(s.Chained, globals, nonlocals, addName)
		}
	case *AugmentedAssignStatement:
		addLocal(s.Name.Value())
	case *MultipleAssignStatement:
		for _, name := range s.Names {
			addLocal(name.Value())
		}
	case *FunctionStatement:
		addLocal(s.Name.Value())
	case *ClassStatement:
		addLocal(s.Name.Value())
	case *ForStatement:
		for _, variable := range s.Variables {
			collectAssignedNamesFromExpression(variable, addLocal)
		}
		collectAssignedNamesFromBlock(s.Body, globals, nonlocals, addName)
		collectAssignedNamesFromBlock(s.Else, globals, nonlocals, addName)
	case *IfStatement:
		collectAssignedNamesFromBlock(s.Consequence, globals, nonlocals, addName)
		for _, clause := range s.ElifClauses {
			collectAssignedNamesFromBlock(clause.Consequence, globals, nonlocals, addName)
		}
		collectAssignedNamesFromBlock(s.Alternative, globals, nonlocals, addName)
	case *WhileStatement:
		collectAssignedNamesFromBlock(s.Body, globals, nonlocals, addName)
		collectAssignedNamesFromBlock(s.Else, globals, nonlocals, addName)
	case *TryStatement:
		collectAssignedNamesFromBlock(s.Body, globals, nonlocals, addName)
		for _, clause := range s.ExceptClauses {
			if clause.ExceptVar != nil {
				addLocal(clause.ExceptVar.Value())
			}
			collectAssignedNamesFromBlock(clause.Body, globals, nonlocals, addName)
		}
		collectAssignedNamesFromBlock(s.Else, globals, nonlocals, addName)
		collectAssignedNamesFromBlock(s.Finally, globals, nonlocals, addName)
	case *WithStatement:
		if s.Target != nil {
			addLocal(s.Target.Value())
		}
		collectAssignedNamesFromBlock(s.Body, globals, nonlocals, addName)
	case *ImportStatement:
		if s.GetAlias() != nil {
			addLocal(s.GetAlias().Value())
		} else if s.Name != nil {
			addLocal(strings.Split(s.Name.Value(), ".")[0])
		}
		for i, name := range s.GetAdditionalNames() {
			if i < len(s.GetAdditionalAliases()) && s.GetAdditionalAliases()[i] != nil {
				addLocal(s.GetAdditionalAliases()[i].Value())
			} else if name != nil {
				addLocal(strings.Split(name.Value(), ".")[0])
			}
		}
	case *FromImportStatement:
		for i, name := range s.Names {
			if i < len(s.Aliases) && s.Aliases[i] != nil {
				addLocal(s.Aliases[i].Value())
			} else if name != nil {
				addLocal(name.Value())
			}
		}
	case *MatchStatement:
		for _, caseClause := range s.Cases {
			if caseClause.CaptureAs != nil {
				addLocal(caseClause.CaptureAs.Value())
			}
			collectAssignedNamesFromBlock(caseClause.Body, globals, nonlocals, addName)
		}
	}
}

func collectAssignedNamesFromExpression(expr Expression, addName func(string)) {
	switch e := expr.(type) {
	case *Identifier:
		addName(e.Value())
	case *TupleLiteral:
		for _, elem := range e.Elements {
			collectAssignedNamesFromExpression(elem, addName)
		}
	case *ListLiteral:
		for _, elem := range e.Elements {
			collectAssignedNamesFromExpression(elem, addName)
		}
	}
}
