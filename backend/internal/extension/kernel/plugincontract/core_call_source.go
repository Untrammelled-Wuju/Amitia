package plugincontract

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type CallSite struct {
	Target     string `json:"target"`
	SideEffect string `json:"sideEffect,omitempty"`
}

type FunctionSurface struct {
	Name  string     `json:"name"`
	File  string     `json:"file"`
	Calls []CallSite `json:"calls"`
}

type RouteRegistration struct {
	Method       string `json:"method"`
	Path         string `json:"path"`
	PathLiteral  bool   `json:"pathLiteral"`
	Package      string `json:"package"`
	PackageDir   string `json:"packageDir"`
	Owner        string `json:"owner"`
	ClosureIndex int    `json:"closureIndex"`
	File         string `json:"file"`
	Line         int    `json:"line"`
	HandlerText  string `json:"handlerText"`
	Handler      *FunctionSurface
}

type CoreCallSources struct {
	Functions     map[string]*FunctionSurface
	Registrations []RouteRegistration
	Files         int
	FunctionsRead int
}

var skippedSourceDirs = map[string]bool{
	"vendor":       true,
	"node_modules": true,
	"testdata":     true,
	".git":         true,
}

var skippedRootDirs = map[string]bool{
	"artifacts": true,
	"build":     true,
	"log":       true,
	"node":      true,
	"runtime":   true,
	"surrealdb": true,
}

var ginRouteMethods = map[string]bool{
	"GET":     true,
	"POST":    true,
	"PUT":     true,
	"PATCH":   true,
	"DELETE":  true,
	"HEAD":    true,
	"OPTIONS": true,
	"Any":     true,
}

func LoadCoreCallSources(root string) (*CoreCallSources, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("plugincontract: resolve source root %q: %w", root, err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return nil, fmt.Errorf("plugincontract: stat source root %q: %w", absolute, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("plugincontract: source root %q is not a directory", absolute)
	}
	sources := &CoreCallSources{Functions: map[string]*FunctionSurface{}}
	fset := token.NewFileSet()
	err = filepath.WalkDir(absolute, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path == absolute {
				return nil
			}
			if skippedSourceDirs[entry.Name()] || strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			if relative, err := filepath.Rel(absolute, path); err == nil {
				if !strings.Contains(filepath.ToSlash(relative), "/") && skippedRootDirs[entry.Name()] {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		relative, err := filepath.Rel(absolute, path)
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return fmt.Errorf("plugincontract: parse %s: %w", relative, err)
		}
		sources.Files++
		sources.collectFile(filepath.ToSlash(relative), file, fset)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(sources.Registrations, func(i, j int) bool {
		left := sources.Registrations[i]
		right := sources.Registrations[j]
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Method != right.Method {
			return left.Method < right.Method
		}
		return left.File < right.File
	})
	return sources, nil
}

func (s *CoreCallSources) collectFile(relative string, file *ast.File, fset *token.FileSet) {
	if s.Functions == nil {
		s.Functions = map[string]*FunctionSurface{}
	}
	packageDir := packageDirKey(relative)
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		name := function.Name.Name
		if function.Recv != nil && len(function.Recv.List) > 0 {
			name = receiverTypeName(function.Recv.List[0].Type) + "." + name
		}
		key := packageDir + "::" + name
		s.indexSurface(key, name, relative, function.Body)
		s.walkFunction(key, packageDir, relative, file.Name.Name, function.Body, fset)
	}
}

func (s *CoreCallSources) indexSurface(key, name, relative string, body *ast.BlockStmt) *FunctionSurface {
	surface := &FunctionSurface{Name: name, File: relative, Calls: collectCallSites(body)}
	s.Functions[key] = surface
	s.FunctionsRead++
	return surface
}

func (s *CoreCallSources) walkFunction(key, packageDir, relative, packageName string, body *ast.BlockStmt, fset *token.FileSet) {
	if body == nil {
		return
	}
	closures := make(map[token.Pos]*FunctionSurface, 8)
	for position, literal := range directFuncLits(body) {
		closureKey := fmt.Sprintf("%s.func%d", key, position+1)
		name := closureKey[strings.Index(closureKey, "::")+2:]
		closures[literal.Pos()] = s.indexSurface(closureKey, name, relative, literal.Body)
		s.walkFunction(closureKey, packageDir, relative, packageName, literal.Body, fset)
	}
	ast.Inspect(body, func(node ast.Node) bool {
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !ginRouteMethods[selector.Sel.Name] {
			return true
		}
		path := ""
		pathLiteral := false
		if literal, ok := call.Args[0].(*ast.BasicLit); ok && literal.Kind == token.STRING {
			if unquoted, err := strconv.Unquote(literal.Value); err == nil {
				path = unquoted
				pathLiteral = true
			}
		}
		handler := call.Args[len(call.Args)-1]
		registration := RouteRegistration{
			Method:      selector.Sel.Name,
			Path:        path,
			PathLiteral: pathLiteral,
			Package:     packageName,
			PackageDir:  packageDir,
			Owner:       ownerName(key),
			File:        relative,
			HandlerText: renderExpression(handler),
			Line:        fset.Position(call.Pos()).Line,
		}
		if closure, ok := handler.(*ast.FuncLit); ok {
			registration.Handler = closures[closure.Pos()]
			registration.ClosureIndex = closureIndex(registration.Handler)
		}
		s.Registrations = append(s.Registrations, registration)
		return true
	})
}

func directFuncLits(body *ast.BlockStmt) []*ast.FuncLit {
	if body == nil {
		return nil
	}
	literals := make([]*ast.FuncLit, 0, 8)
	ast.Inspect(body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.FuncLit:
			literals = append(literals, typed)
			return false
		default:
			return true
		}
	})
	return literals
}

func ownerName(key string) string {
	if index := strings.Index(key, "::"); index >= 0 {
		return key[index+2:]
	}
	return key
}

func closureIndex(surface *FunctionSurface) int {
	if surface == nil {
		return 0
	}
	position := strings.LastIndex(surface.Name, ".func")
	if position < 0 {
		return 0
	}
	value, err := strconv.Atoi(surface.Name[position+5:])
	if err != nil {
		return 0
	}
	return value
}

func packageDirKey(relative string) string {
	directory := filepath.ToSlash(filepath.Dir(relative))
	if directory == "." {
		return ""
	}
	return directory
}

func (s *CoreCallSources) FindFunction(packageDir, name string) (*FunctionSurface, bool) {
	if s == nil || s.Functions == nil {
		return nil, false
	}
	surface, ok := s.Functions[strings.Trim(packageDir, "/")+"::"+name]
	return surface, ok
}

func receiverTypeName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.StarExpr:
		return receiverTypeName(typed.X)
	case *ast.Ident:
		return typed.Name
	case *ast.IndexExpr:
		return receiverTypeName(typed.X)
	default:
		return renderExpression(expression)
	}
}

func collectCallSites(body *ast.BlockStmt) []CallSite {
	if body == nil {
		return nil
	}
	seen := map[string]bool{}
	sites := make([]CallSite, 0, 16)
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		target := renderExpression(call.Fun)
		if target == "" || seen[target] || goBuiltins[target] {
			return true
		}
		seen[target] = true
		sites = append(sites, CallSite{Target: target, SideEffect: sideEffectFor(target)})
		return true
	})
	sort.Slice(sites, func(i, j int) bool { return sites[i].Target < sites[j].Target })
	return sites
}

func renderExpression(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		parent := renderExpression(typed.X)
		if parent == "" {
			return typed.Sel.Name
		}
		return parent + "." + typed.Sel.Name
	case *ast.CallExpr:
		return renderExpression(typed.Fun) + "()"
	case *ast.IndexExpr:
		return renderExpression(typed.X)
	case *ast.StarExpr:
		return "*" + renderExpression(typed.X)
	case *ast.ParenExpr:
		return renderExpression(typed.X)
	case *ast.FuncLit:
		return "(func)"
	case *ast.BasicLit:
		return typed.Value
	case *ast.UnaryExpr:
		return typed.Op.String() + renderExpression(typed.X)
	default:
		return ""
	}
}

var goBuiltins = map[string]bool{
	"append": true, "cap": true, "clear": true, "close": true, "complex": true, "copy": true,
	"delete": true, "imag": true, "len": true, "make": true, "max": true, "min": true,
	"new": true, "panic": true, "print": true, "println": true, "real": true, "recover": true,
	"string": true, "bool": true, "byte": true, "rune": true, "int": true, "int32": true,
	"int64": true, "float32": true, "float64": true, "uint": true, "uint8": true, "uint64": true,
	"any": true, "error": true,
}

var sideEffectTokens = []struct {
	Token string
	Kind  string
}{
	{Token: "Insert", Kind: "db.write"},
	{Token: "Upsert", Kind: "db.write"},
	{Token: "Create", Kind: "db.write"},
	{Token: "Update", Kind: "db.write"},
	{Token: "Delete", Kind: "db.write"},
	{Token: "Remove", Kind: "db.write"},
	{Token: "Save", Kind: "db.write"},
	{Token: "Commit", Kind: "db.write"},
	{Token: "Query", Kind: "db.read"},
	{Token: "Find", Kind: "db.read"},
	{Token: "Select", Kind: "db.read"},
	{Token: "List", Kind: "db.read"},
	{Token: "Count", Kind: "db.read"},
	{Token: "WriteFile", Kind: "file.write"},
	{Token: "MkdirAll", Kind: "file.write"},
	{Token: "OpenFile", Kind: "file.write"},
	{Token: "Command", Kind: "process.exec"},
	{Token: "StartProcess", Kind: "process.exec"},
	{Token: "Flush", Kind: "stream.write"},
}

func sideEffectFor(target string) string {
	last := target
	if index := strings.LastIndex(target, "."); index >= 0 {
		last = target[index+1:]
	}
	for _, candidate := range sideEffectTokens {
		if candidate.Token == last {
			return candidate.Kind
		}
	}
	return ""
}
