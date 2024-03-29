module nar-compiler

go 1.21

//replace (
//	github.com/nar-lang/nar-common v1.0.1 => ../nar-common
//	github.com/nar-lang/nar-compiler v1.0.1 => ../nar-compiler
//	github.com/nar-lang/nar-lsp v1.0.1 => ../nar-lsp
//)

require github.com/nar-lang/nar-common v1.0.1

require github.com/nar-lang/nar-compiler v1.0.1

require github.com/nar-lang/nar-lsp v1.0.1

require golang.org/x/text v0.14.0 // indirect
