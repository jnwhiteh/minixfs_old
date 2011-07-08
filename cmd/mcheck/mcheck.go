package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
)

var fset *token.FileSet = token.NewFileSet()

type astVisitor func(n ast.Node) bool

func (f astVisitor) Visit(n ast.Node) ast.Visitor {
	if f(n) {
		return f
	}
	return nil
}

var typeVisitor astVisitor = func(n ast.Node) bool {
	if n == nil {
		return false
	}

	pos := fset.Position(n.Pos())
	debugf("%s: Visiting %q %T", pos, n, n)
	return true
}

var typeMatch *string = flag.String("type", "Superblock", "the type for which you want to scan")
var showDebug *bool = flag.Bool("debug", true, "display debug information")

func main() {
	flag.Parse()

	pkgs, err := parser.ParseFiles(fset, flag.Args(), 0)
	if err != nil {
		fmt.Printf("Error when parsing source: %s", err)
	}

	for _, pkg := range pkgs {
		fmt.Printf("Checking package '%s'\n", pkg.Name)
		pkg, _ := ast.NewPackage(fset, pkg.Files, types.GcImporter, types.Universe)
		types.Check(fset, pkg)
		if err != nil {
			fmt.Printf("Error during type-checking: %s", err)
			return
		}

		ast.Fprint(os.Stdout, fset, pkg, nil)
//		for _, f := range pkg.Files {
//			fmt.Printf("Scanning %s\n", fset.Position(f.Package).Filename)
//			ast.Walk(typeVisitor, f)
//		}
	}
}
