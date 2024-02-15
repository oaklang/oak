package parsed

import (
	"nar-compiler/internal/pkg/ast"
	"nar-compiler/internal/pkg/ast/normalized"
)

func NewPNamed(loc ast.Location, name ast.Identifier) Pattern {
	return &PNamed{
		patternBase: newPatternBase(loc),
		name:        name,
	}
}

type PNamed struct {
	*patternBase
	name ast.Identifier
}

func (e *PNamed) Iterate(f func(statement Statement)) {
	f(e)
	e.patternBase.Iterate(f)
}

func (e *PNamed) Name() ast.Identifier {
	return e.name
}

func (e *PNamed) normalize(
	locals map[ast.Identifier]normalized.Pattern,
	modules map[ast.QualifiedIdentifier]*Module,
	module *Module,
	normalizedModule *normalized.Module,
) (normalized.Pattern, error) {
	var declaredType normalized.Type
	var err error
	if e.declaredType != nil {
		declaredType, err = e.declaredType.normalize(modules, module, nil)
	}
	np := normalized.NewPNamed(e.location, declaredType, e.name)
	locals[e.name] = np
	return e.setSuccessor(np), err
}
