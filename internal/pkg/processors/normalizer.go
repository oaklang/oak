package processors

import (
	"fmt"
	"oak-compiler/internal/pkg/ast"
	"oak-compiler/internal/pkg/ast/normalized"
	"oak-compiler/internal/pkg/ast/parsed"
	"oak-compiler/internal/pkg/common"
	"slices"
	"strings"
)

var lastDefinitionId = uint64(0)

func Normalize(
	moduleName ast.QualifiedIdentifier,
	modules map[ast.QualifiedIdentifier]parsed.Module,
	normalizedModules map[ast.QualifiedIdentifier]*normalized.Module,
) {
	if _, ok := normalizedModules[moduleName]; ok {
		return
	}

	m := modules[moduleName]

	for _, imp := range m.Imports {
		Normalize(imp.ModuleIdentifier, modules, normalizedModules)
	}

	flattenDataTypes(&m)
	unwrapImports(&m, modules)
	modules[moduleName] = m

	o := &normalized.Module{
		Name: m.Name,
	}

	for _, def := range m.Definitions {
		o.Definitions = append(o.Definitions, normalizeDefinition(modules, m, def))
	}

	for _, imp := range m.Imports {
		o.Dependencies = append(o.Dependencies, imp.ModuleIdentifier)
	}

	normalizedModules[m.Name] = o
}

func flattenDataTypes(m *parsed.Module) {
	for _, it := range m.DataTypes {
		typeArgs := common.Map(func(x ast.Identifier) parsed.Type {
			return parsed.TTypeParameter{
				Location: it.Location,
				Name:     x,
			}
		}, it.Params)
		m.Aliases = append(m.Aliases, parsed.Alias{
			Location: it.Location,
			Name:     it.Name,
			Params:   it.Params,
			Type: parsed.TData{
				Location: it.Location,
				Name:     common.MakeExternalIdentifier(m.Name, it.Name),
				Args:     typeArgs,
				Options:  common.Map(func(x parsed.DataTypeOption) ast.Identifier { return x.Name }, it.Options),
			},
		})
		for _, option := range it.Options {
			var type_ parsed.Type = parsed.TExternal{
				Name: common.MakeExternalIdentifier(m.Name, it.Name),
				Args: typeArgs,
			}
			if len(option.Params) > 0 {
				type_ = parsed.TFunc{
					Location: it.Location,
					Params:   option.Params,
					Return:   type_,
				}
			}
			var body parsed.Expression = parsed.Constructor{
				Location:   option.Location,
				DataName:   common.MakeExternalIdentifier(m.Name, it.Name),
				OptionName: option.Name,
				Args: common.Map(
					func(i int) parsed.Expression {
						return parsed.Var{
							Location: option.Location,
							Name:     ast.QualifiedIdentifier(fmt.Sprintf("p%d", i)),
						}
					},
					common.Range(0, len(option.Params)),
				),
			}

			params := common.Map(
				func(i int) parsed.Pattern {
					return parsed.PNamed{Location: option.Location, Name: ast.Identifier(fmt.Sprintf("p%d", i))}
				},
				common.Range(0, len(option.Params)),
			)

			m.Definitions = append(m.Definitions, parsed.Definition{
				Location:   option.Location,
				Hidden:     option.Hidden || it.Hidden,
				Name:       option.Name,
				Params:     params,
				Expression: body,
				Type:       type_,
			})
		}
	}
}

func unwrapImports(module *parsed.Module, modules map[ast.QualifiedIdentifier]parsed.Module) {
	for i, imp := range module.Imports {
		m := modules[imp.ModuleIdentifier]
		modName := m.Name
		if imp.Alias != nil {
			modName = ast.QualifiedIdentifier(*imp.Alias)
		}
		shortModName := ast.QualifiedIdentifier("")
		lastDotIndex := strings.LastIndex(string(modName), ".")
		if lastDotIndex >= 0 {
			shortModName = modName[lastDotIndex+1:]
		}

		var exp []string

		for _, d := range m.Definitions {
			n := string(d.Name)
			if imp.ExposingAll || slices.Contains(imp.Exposing, n) {
				exp = append(exp, n)
			}
			exp = append(exp, fmt.Sprintf("%s.%s", modName, n))
			if shortModName != "" {
				exp = append(exp, fmt.Sprintf("%s.%s", shortModName, n))
			}
		}

		for _, a := range m.Aliases {
			n := string(a.Name)
			if imp.ExposingAll || slices.Contains(imp.Exposing, n) {
				exp = append(exp, n)
				if dt, ok := a.Type.(parsed.TData); ok {
					for _, v := range dt.Options {
						exp = append(exp, string(v))
					}
				}
			}
			exp = append(exp, fmt.Sprintf("%s.%s", modName, n))
			if dt, ok := a.Type.(parsed.TData); ok {
				for _, v := range dt.Options {
					exp = append(exp, fmt.Sprintf("%s.%s", modName, v))
					if shortModName != "" {
						exp = append(exp, fmt.Sprintf("%s.%s", shortModName, v))
					}
				}
			}
		}

		for _, a := range m.InfixFns {
			n := string(a.Name)
			if imp.ExposingAll || slices.Contains(imp.Exposing, n) {
				exp = append(exp, n)
			}
			exp = append(exp, fmt.Sprintf("%s.%s", modName, n))
			if shortModName != "" {
				exp = append(exp, fmt.Sprintf("%s.%s", shortModName, n))
			}
		}
		imp.Exposing = exp
		module.Imports[i] = imp
	}
}

func normalizeDefinition(
	modules map[ast.QualifiedIdentifier]parsed.Module, module parsed.Module, def parsed.Definition,
) normalized.Definition {
	lastDefinitionId++
	o := normalized.Definition{
		Id:       lastDefinitionId,
		Name:     def.Name,
		Location: def.Location,
		Hidden:   def.Hidden,
	}
	o.Params = common.Map(func(x parsed.Pattern) normalized.Pattern {
		return normalizePattern(modules, module, x)
	}, def.Params)
	o.Expression = normalizeExpression(modules, module, def.Expression)
	o.Type = normalizeType(modules, module, def.Type)
	return o
}

func normalizePattern(
	modules map[ast.QualifiedIdentifier]parsed.Module, module parsed.Module, pattern parsed.Pattern,
) normalized.Pattern {
	normalize := func(p parsed.Pattern) normalized.Pattern { return normalizePattern(modules, module, p) }

	switch pattern.(type) {
	case parsed.PAlias:
		{
			e := pattern.(parsed.PAlias)
			return normalized.PAlias{
				Location: e.Location,
				Type:     normalizeType(modules, module, e.Type),
				Alias:    e.Alias,
				Nested:   normalize(e.Nested),
			}
		}
	case parsed.PAny:
		{
			e := pattern.(parsed.PAny)
			return normalized.PAny{
				Location: e.Location,
				Type:     normalizeType(modules, module, e.Type),
			}
		}
	case parsed.PCons:
		{
			e := pattern.(parsed.PCons)
			return normalized.PCons{
				Location: e.Location,
				Type:     normalizeType(modules, module, e.Type),
				Head:     normalize(e.Head),
				Tail:     normalize(e.Tail),
			}
		}
	case parsed.PConst:
		{
			e := pattern.(parsed.PConst)
			return normalized.PConst{
				Location: e.Location,
				Type:     normalizeType(modules, module, e.Type),
				Value:    e.Value,
			}
		}
	case parsed.PDataOption:
		{
			e := pattern.(parsed.PDataOption)
			mod, def, ok := findParsedDefinition(modules, module, e.Name)
			if !ok {
				panic(common.Error{Location: e.Location, Message: "data constructor not found"})
			}
			return normalized.PDataOption{
				Location:       e.Location,
				Type:           normalizeType(modules, module, e.Type),
				ModuleName:     mod.Name,
				DefinitionName: def.Name,
				Values:         common.Map(normalize, e.Values),
			}
		}
	case parsed.PList:
		{
			e := pattern.(parsed.PList)
			return normalized.PList{
				Location: e.Location,
				Type:     normalizeType(modules, module, e.Type),
				Items:    common.Map(normalize, e.Items),
			}
		}
	case parsed.PNamed:
		{
			e := pattern.(parsed.PNamed)
			return normalized.PNamed{
				Location: e.Location,
				Type:     normalizeType(modules, module, e.Type),
				Name:     e.Name,
			}
		}
	case parsed.PRecord:
		{
			e := pattern.(parsed.PRecord)
			return normalized.PRecord{
				Location: e.Location,
				Type:     normalizeType(modules, module, e.Type),
				Fields: common.Map(func(x parsed.PRecordField) normalized.PRecordField {
					return normalized.PRecordField{Location: x.Location, Name: x.Name}
				}, e.Fields),
			}
		}
	case parsed.PTuple:
		{
			e := pattern.(parsed.PTuple)
			return normalized.PTuple{
				Location: e.Location,
				Type:     normalizeType(modules, module, e.Type),
				Items:    common.Map(normalize, e.Items),
			}
		}
	}
	panic(common.SystemError{Message: "impossible case"})
}

func normalizeExpression(
	modules map[ast.QualifiedIdentifier]parsed.Module, module parsed.Module, expr parsed.Expression,
) normalized.Expression {
	normalize := func(e parsed.Expression) normalized.Expression {
		return normalizeExpression(modules, module, e)
	}
	switch expr.(type) {
	case parsed.Access:
		{
			e := expr.(parsed.Access)
			return normalized.Access{
				Location:  e.Location,
				Record:    normalize(e.Record),
				FieldName: e.FieldName,
			}
		}
	case parsed.Apply:
		{
			e := expr.(parsed.Apply)
			return normalized.Apply{
				Location: e.Location,
				Func:     normalize(e.Func),
				Args:     common.Map(normalize, e.Args),
			}
		}
	case parsed.Const:
		{
			e := expr.(parsed.Const)
			return normalized.Const{
				Location: e.Location,
				Value:    e.Value,
			}
		}
	case parsed.Constructor:
		{
			e := expr.(parsed.Constructor)
			return normalized.Constructor{
				Location:   e.Location,
				DataName:   e.DataName,
				OptionName: e.OptionName,
				Args:       common.Map(normalize, e.Args),
			}
		}
	case parsed.If:
		{
			e := expr.(parsed.If)
			return normalized.If{
				Location:  e.Location,
				Condition: normalize(e.Condition),
				Positive:  normalize(e.Positive),
				Negative:  normalize(e.Negative),
			}
		}
	case parsed.Let:
		{
			e := expr.(parsed.Let)
			return normalized.Let{
				Location: e.Location,
				Pattern:  normalizePattern(modules, module, e.Pattern),
				Value:    normalize(e.Value),
				Body:     normalize(e.Body),
			}
		}
	case parsed.List:
		{
			e := expr.(parsed.List)
			return normalized.List{
				Location: e.Location,
				Items:    common.Map(normalize, e.Items),
			}
		}
	case parsed.NativeCall:
		{
			e := expr.(parsed.NativeCall)
			return normalized.NativeCall{
				Location: e.Location,
				Name:     e.Name,
				Args:     common.Map(normalize, e.Args),
			}
		}
	case parsed.Record:
		{
			e := expr.(parsed.Record)
			return normalized.Record{
				Location: e.Location,
				Fields: common.Map(func(i parsed.RecordField) normalized.RecordField {
					return normalized.RecordField{
						Location: i.Location,
						Name:     i.Name,
						Value:    normalize(i.Value),
					}
				}, e.Fields),
			}
		}
	case parsed.Select:
		{
			//TODO: Check if all cases are exhausting condition
			e := expr.(parsed.Select)
			return normalized.Select{
				Location:  e.Location,
				Condition: normalize(e.Condition),
				Cases: common.Map(func(i parsed.SelectCase) normalized.SelectCase {
					return normalized.SelectCase{
						Location:   e.Location,
						Pattern:    normalizePattern(modules, module, i.Pattern),
						Expression: normalize(i.Expression),
					}
				}, e.Cases),
			}
		}
	case parsed.Tuple:
		{
			e := expr.(parsed.Tuple)
			return normalized.Tuple{
				Location: e.Location,
				Items:    common.Map(normalize, e.Items),
			}
		}
	case parsed.Update:
		{
			e := expr.(parsed.Update)
			if m, d, ok := findParsedDefinition(modules, module, e.RecordName); ok {
				return normalized.UpdateGlobal{
					Location:       e.Location,
					ModuleName:     m.Name,
					DefinitionName: d.Name,
					Fields: common.Map(func(i parsed.RecordField) normalized.RecordField {
						return normalized.RecordField{
							Location: i.Location,
							Name:     i.Name,
							Value:    normalize(i.Value),
						}
					}, e.Fields),
				}
			}

			return normalized.UpdateLocal{
				Location:   e.Location,
				RecordName: ast.Identifier(e.RecordName),
				Fields: common.Map(func(i parsed.RecordField) normalized.RecordField {
					return normalized.RecordField{
						Location: i.Location,
						Name:     i.Name,
						Value:    normalize(i.Value),
					}
				}, e.Fields),
			}
		}
	case parsed.Lambda:
		{
			e := expr.(parsed.Lambda)
			return normalized.Lambda{
				Location: e.Location,
				Params: common.Map(func(p parsed.Pattern) normalized.Pattern {
					return normalizePattern(modules, module, p)
				}, e.Params),
				Body: normalize(e.Body),
			}
		}
	case parsed.Accessor:
		{
			e := expr.(parsed.Accessor)
			return normalize(parsed.Lambda{
				Params: []parsed.Pattern{parsed.PNamed{Location: e.Location, Name: "x"}},
				Body: parsed.Access{
					Location: e.Location,
					Record: parsed.Var{
						Location: e.Location,
						Name:     "x",
					},
					FieldName: e.FieldName,
				},
			})
		}
	case parsed.BinOp:
		{
			e := expr.(parsed.BinOp)
			var output []parsed.BinOpItem
			var operators []parsed.BinOpItem
			for _, o1 := range e.Items {
				if o1.Expression != nil {
					output = append(output, o1)
				} else {
					if _, infixFn, ok := findInfixFn(modules, module, o1.Infix); !ok {
						panic(common.Error{Location: e.Location, Message: "infix op not found"})
					} else {
						o1.Fn = infixFn
					}

					for i := len(operators) - 1; i >= 0; i-- {
						o2 := operators[i]
						if o2.Fn.Precedence > o1.Fn.Precedence || (o2.Fn.Precedence == o1.Fn.Precedence && o1.Fn.Associativity == parsed.Left) {
							output = append(output, o2)
							operators = operators[:len(operators)-1]
						} else {
							break
						}
					}
					operators = append(operators, o1)
				}
			}
			for i := len(operators) - 1; i >= 0; i-- {
				output = append(output, operators[i])
			}

			var buildTree func() normalized.Expression
			buildTree = func() normalized.Expression {
				op := output[len(output)-1].Infix
				output = output[:len(output)-1]

				if m, infixA, ok := findInfixFn(modules, module, op); !ok {
					panic(common.Error{Location: e.Location, Message: "infix op not found"})
				} else {
					var left, right normalized.Expression
					r := output[len(output)-1]
					if r.Expression != nil {
						right = normalize(r.Expression)
						output = output[:len(output)-1]
					} else {
						right = buildTree()
					}

					l := output[len(output)-1]
					if l.Expression != nil {
						left = normalize(l.Expression)
						output = output[:len(output)-1]
					} else {
						left = buildTree()
					}

					return normalized.Apply{
						Location: e.Location,
						Func: normalized.Var{
							Location:       e.Location,
							ModuleName:     m.Name,
							DefinitionName: infixA.Alias,
						},
						Args: []normalized.Expression{left, right},
					}
				}
			}

			return buildTree()
		}
	case parsed.Negate:
		{
			e := expr.(parsed.Negate)
			return normalized.NativeCall{
				Location: e.Location,
				Name:     common.OakCoreBasicsNeg,
				Args:     []normalized.Expression{normalize(e.Nested)},
			}
		}
	case parsed.Var:
		{
			e := expr.(parsed.Var)
			o := normalized.Var{
				Location: e.Location,
				Name:     e.Name,
			}
			if m, d, ok := findParsedDefinition(modules, module, e.Name); ok {
				o.ModuleName = m.Name
				o.DefinitionName = d.Name
			}
			return o
		}
	case parsed.InfixVar:
		{
			e := expr.(parsed.InfixVar)
			if m, i, ok := findInfixFn(modules, module, e.Infix); !ok {
				panic(common.Error{
					Location: i.AliasLocation,
					Message:  "infix definition not found",
				})
			} else if _, d, ok := findParsedDefinition(nil, m, ast.QualifiedIdentifier(i.Alias)); !ok {
				panic(common.Error{
					Location: i.Location,
					Message:  "infix alias not found",
				})
			} else {
				return normalized.Var{
					Location:       e.Location,
					ModuleName:     m.Name,
					DefinitionName: d.Name,
				}
			}
		}
	}
	panic(common.SystemError{Message: "impossible case"})
}

func normalizeType(
	modules map[ast.QualifiedIdentifier]parsed.Module, module parsed.Module, t parsed.Type,
) normalized.Type {
	if t == nil {
		return nil //TODO: find places where it can happen and check there
	}
	normalize := func(x parsed.Type) normalized.Type {
		return normalizeType(modules, module, x)
	}
	switch t.(type) {
	case parsed.TFunc:
		{
			e := t.(parsed.TFunc)
			return normalized.TFunc{
				Location: e.Location,
				Params:   common.Map(normalize, e.Params),
				Return:   normalize(e.Return),
			}
		}
	case parsed.TRecord:
		{
			e := t.(parsed.TRecord)
			fields := map[ast.Identifier]normalized.Type{}
			for n, v := range e.Fields {
				fields[n] = normalize(v)
			}
			return normalized.TRecord{
				Location: e.Location,
				Fields:   fields,
			}
		}
	case parsed.TTuple:
		{
			e := t.(parsed.TTuple)
			return normalized.TTuple{
				Location: e.Location,
				Items:    common.Map(normalize, e.Items),
			}
		}
	case parsed.TUnit:
		{
			e := t.(parsed.TUnit)
			return normalized.TUnit{
				Location: e.Location,
			}
		}
	case parsed.TData:
		{
			e := t.(parsed.TData)
			return normalized.TData{
				Location: e.Location,
				Name:     e.Name,
				Args:     common.Map(normalize, e.Args),
			}
		}
	case parsed.TExternal:
		{
			e := t.(parsed.TExternal)
			return normalized.TExternal{
				Location: e.Location,
				Name:     e.Name,
				Args:     common.Map(normalize, e.Args),
			}
		}
	case parsed.TTypeParameter:
		{
			e := t.(parsed.TTypeParameter)
			return normalized.TTypeParameter{
				Location: e.Location,
				Name:     e.Name,
			}
		}
	case parsed.TNamed:
		{
			e := t.(parsed.TNamed)
			x, ok := findParsedType(modules, module, e.Name, e.Args)
			if !ok {
				panic(common.Error{Location: e.Location, Message: "type not found"})
			}
			return normalizeType(modules, module, x)
		}
	}
	panic(common.SystemError{Message: "impossible case"})
}

func findParsedDefinition(
	modules map[ast.QualifiedIdentifier]parsed.Module, module parsed.Module, name ast.QualifiedIdentifier,
) (parsed.Module, parsed.Definition, bool) {
	var defNameEq = func(x parsed.Definition) bool {
		return ast.QualifiedIdentifier(x.Name) == name
	}

	if def, ok := common.Find(defNameEq, module.Definitions); ok {
		return module, def, true
	}

	ids := strings.Split(string(name), ".")
	defName := ast.QualifiedIdentifier(ids[len(ids)-1])

	for _, imp := range module.Imports {
		if slices.Contains(imp.Exposing, string(name)) {
			return findParsedDefinition(nil, modules[imp.ModuleIdentifier], defName)
		}
	}

	return parsed.Module{}, parsed.Definition{}, false
}

func findInfixFn(
	modules map[ast.QualifiedIdentifier]parsed.Module, module parsed.Module, name ast.InfixIdentifier,
) (parsed.Module, parsed.Infix, bool) {
	var infNameEq = func(x parsed.Infix) bool { return x.Name == name }
	if inf, ok := common.Find(infNameEq, module.InfixFns); ok {
		return module, inf, true
	}

	for _, imp := range module.Imports {
		if slices.Contains(imp.Exposing, string(name)) {
			return findInfixFn(nil, modules[imp.ModuleIdentifier], name)
		}
	}
	return parsed.Module{}, parsed.Infix{}, false
}

func findParsedType(
	modules map[ast.QualifiedIdentifier]parsed.Module,
	module parsed.Module,
	name ast.QualifiedIdentifier,
	args []parsed.Type,
) (parsed.Type, bool) {
	var aliasNameEq = func(x parsed.Alias) bool {
		return ast.QualifiedIdentifier(x.Name) == name
	}

	if alias, ok := common.Find(aliasNameEq, module.Aliases); ok {
		if alias.Type == nil {
			return parsed.TExternal{
				Location: alias.Location,
				Name:     common.MakeExternalIdentifier(module.Name, alias.Name),
				Args:     args,
			}, true
		}
		return applyTypeArgs(alias.Type, args)
	}

	ids := strings.Split(string(name), ".")
	typeName := ast.QualifiedIdentifier(ids[len(ids)-1])

	for _, imp := range module.Imports {
		if slices.Contains(imp.Exposing, string(name)) {
			return findParsedType(nil, modules[imp.ModuleIdentifier], typeName, args)
		}
	}

	return nil, false
}

func applyTypeArgs(t parsed.Type, args []parsed.Type) (parsed.Type, bool) {
	switch t.(type) {
	case parsed.TFunc:
		return t, true
	case parsed.TRecord:
		return t, true
	case parsed.TTuple:
		return t, true
	case parsed.TUnit:
		return t, true
	case parsed.TData:
		{
			e := t.(parsed.TData)
			if len(e.Args) != len(args) {
				return nil, false
			}
			e.Args = args
			return e, true
		}
	case parsed.TNamed:
		{
			e := t.(parsed.TNamed)
			if len(e.Args) != len(args) {
				return nil, false
			}
			e.Args = args
			return e, true
		}
	case parsed.TExternal:
		{
			e := t.(parsed.TExternal)
			if len(e.Args) != len(args) {
				return nil, false
			}
			e.Args = args
			return e, true
		}
	case parsed.TTypeParameter:
		{
			return t, true
		}
	}
	panic(common.SystemError{Message: "impossible case"})
}
