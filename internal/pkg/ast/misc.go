package ast

import "strings"

type Identifier string

type QualifiedIdentifier string

type PackageIdentifier string

type InfixIdentifier string

type FullIdentifier string

func (f FullIdentifier) String() string {
	return string(f)
}

func (f FullIdentifier) Module() QualifiedIdentifier {
	return QualifiedIdentifier(f[:strings.LastIndex(string(f), ".")])
}

type DataOptionIdentifier string

type Location struct {
	FilePath    string
	FileContent []rune
	Position    uint32
}

func (loc Location) GetLineAndColumn() (line int, column int) {
	line = 1
	column = 1

	p := len(loc.FileContent)
	if p > int(loc.Position) {
		p = int(loc.Position)
	}
	for i := 0; i < p; i++ {
		if '\n' == loc.FileContent[i] {
			line++
			column = 1
		} else {
			column++
		}
	}
	return
}
