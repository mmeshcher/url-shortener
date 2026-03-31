package linter

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "linter",
	Doc:  "reports calls to panic, os.Exit, and log.Fatal outside of main.main",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	isMainPkg := pass.Pkg.Name() == "main"

	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			// Check for function declarations to identify main.main
			if fn, ok := n.(*ast.FuncDecl); ok {
				if isMainPkg && fn.Name.Name == "main" {
					// We are inside main.main, os.Exit and log.Fatal are allowed here.
					// But we still need to check for panic.
					if fn.Body != nil {
						ast.Inspect(fn.Body, func(inner ast.Node) bool {
							checkPanic(pass, inner)
							return true
						})
					}
					return false // Skip further inspection of this function by the outer Inspect
				}
			}

			// Check for panic, os.Exit, and log.Fatal in all other contexts
			checkPanic(pass, n)
			checkExitAndFatal(pass, n)

			return true
		})
	}

	return nil, nil
}

func checkPanic(pass *analysis.Pass, n ast.Node) {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return
	}

	ident, ok := call.Fun.(*ast.Ident)
	if ok && ident.Name == "panic" && ident.Obj == nil {
		pass.Reportf(call.Pos(), "usage of panic is prohibited")
	}
}

func checkExitAndFatal(pass *analysis.Pass, n ast.Node) {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return
	}

	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	pkgIdent, ok := selector.X.(*ast.Ident)
	if !ok {
		return
	}

	// Check for os.Exit
	if pkgIdent.Name == "os" && selector.Sel.Name == "Exit" {
		pass.Reportf(call.Pos(), "usage of os.Exit is prohibited outside of main.main")
	}

	// Check for log.Fatal, log.Fatalf, log.Fatalln
	if pkgIdent.Name == "log" && (selector.Sel.Name == "Fatal" || selector.Sel.Name == "Fatalf" || selector.Sel.Name == "Fatalln") {
		pass.Reportf(call.Pos(), "usage of log.Fatal is prohibited outside of main.main")
	}
}
