package parsed

import (
	"fmt"
	"nar-compiler/internal/pkg/ast"
	"nar-compiler/internal/pkg/ast/normalized"
	"nar-compiler/internal/pkg/common"
)

func NewPOption(loc ast.Location, name ast.QualifiedIdentifier, values []Pattern) Pattern {
	return &POption{
		patternBase: newPatternBase(loc),
		name:        name,
		values:      values,
	}
}

type POption struct {
	*patternBase
	name   ast.QualifiedIdentifier
	values []Pattern
}

func (e *POption) Iterate(f func(statement Statement)) {
	f(e)
	for _, value := range e.values {
		if value != nil {
			value.Iterate(f)
		}
	}
	e.patternBase.Iterate(f)
}

func (e *POption) normalize(
	locals map[ast.Identifier]normalized.Pattern,
	modules map[ast.QualifiedIdentifier]*Module,
	module *Module,
	normalizedModule *normalized.Module,
) (normalized.Pattern, error) {
	def, mod, ids := module.findDefinitionAndAddDependency(modules, e.name, normalizedModule)
	if len(ids) == 0 {
		return nil, common.Error{Location: e.location, Message: "data constructor not found"}
	} else if len(ids) > 1 {
		return nil, common.Error{
			Location: e.location,
			Message: fmt.Sprintf(
				"ambiguous data constructor `%s`, it can be one of %s. "+
					"Use import or qualified identifer to clarify which one to use",
				e.name, common.Join(ids, ", ")),
		}
	}
	var values []normalized.Pattern
	var errors []error
	for _, value := range e.values {
		nValue, err := value.normalize(locals, modules, module, normalizedModule)
		errors = append(errors, err)
		values = append(values, nValue)
	}

	var declaredType normalized.Type
	if e.declaredType != nil {
		var err error
		declaredType, err = e.declaredType.normalize(modules, module, nil)
		errors = append(errors, err)
	}
	return e.setSuccessor(normalized.NewPOption(e.location, declaredType, mod.name, def.name(), values)),
		common.MergeErrors(errors...)
}
