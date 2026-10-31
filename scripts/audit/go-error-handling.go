package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type replacement struct {
	start int
	end   int
	text  string
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	mode := "audit"
	if len(os.Args) > 2 {
		mode = os.Args[2]
	}
	files := []string{}
	err := filepath.Walk(filepath.Join(root, "backend"), func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			name := info.Name()
			if name == "node_modules" || name == "dist" || name == "build" || name == "artifacts" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	sort.Strings(files)
	findings := []string{}
	changed := 0
	for _, file := range files {
		source, readErr := os.ReadFile(file)
		if readErr != nil {
			panic(readErr)
		}
		fset := token.NewFileSet()
		parsed, parseErr := parser.ParseFile(fset, file, source, parser.ParseComments)
		if parseErr != nil {
			panic(parseErr)
		}
		replacements := []replacement{}
		seenResponses := map[int]bool{}
		logName := existingLogImportName(parsed)
		needsLog := false
		ast.Inspect(parsed, func(node ast.Node) bool {
			assignment, ok := node.(*ast.AssignStmt)
			if !ok || len(assignment.Rhs) != 1 || len(assignment.Lhs) > 2 {
				return true
			}
			allDiscarded := len(assignment.Lhs) > 0
			for _, left := range assignment.Lhs {
				identifier, ok := left.(*ast.Ident)
				if !ok || identifier.Name != "_" {
					allDiscarded = false
					break
				}
			}
			if !allDiscarded {
				return true
			}
			var callEnd ast.Expr = assignment.Rhs[0]
			if call, ok := assignment.Rhs[0].(*ast.CallExpr); ok {
				callEnd = call
			} else if selector, ok := assignment.Rhs[0].(*ast.SelectorExpr); !ok || selector.Sel.Name != "Error" {
				return true
			}
			if logName == "" {
				logName = "amitialog"
			}
			start := fset.Position(assignment.Pos())
			end := fset.Position(assignment.End())
			callText := string(source[start.Offset:fset.Position(callEnd.Pos()).Offset]) + string(source[fset.Position(callEnd.Pos()).Offset:fset.Position(callEnd.End()).Offset])
			findings = append(findings, fmt.Sprintf("%s:%d: ignored error return %s", file, start.Line, callText))
			owner := strings.TrimSpace(string(source[start.Offset:fset.Position(callEnd.Pos()).Offset]))
			if owner != "_ =" && owner != "_, _ =" {
				return true
			}
			leftSide := "err :="
			if len(assignment.Lhs) == 2 {
				leftSide = "_, err :="
			}
			statement := "if " + leftSide + " " + string(source[fset.Position(callEnd.Pos()).Offset:fset.Position(callEnd.End()).Offset]) + "; err != nil { " + logName + `.Printf("[original-error] %v", err) }`
			replacements = append(replacements, replacement{start: start.Offset, end: end.Offset, text: statement})
			needsLog = true
			return true
		})
		ast.Inspect(parsed, func(node ast.Node) bool {
			statement, ok := node.(*ast.IfStmt)
			if !ok {
				return true
			}
			errorName := branchErrorName(statement)
			if errorName == "" {
				return true
			}
			ast.Inspect(statement.Body, func(child ast.Node) bool {
				if composite, ok := child.(*ast.CompositeLit); ok {
					literals := structuredErrorMessageLiterals(composite)
					start := fset.Position(composite.Pos()).Offset
					end := fset.Position(composite.End()).Offset
					if len(literals) > 0 && !strings.Contains(string(source[start:end]), errorName) {
						for _, literal := range literals {
							position := fset.Position(literal.Pos())
							if seenResponses[position.Offset] {
								continue
							}
							seenResponses[position.Offset] = true
							findings = append(findings, fmt.Sprintf("%s:%d: %s", file, position.Line, literal.Value))
							replacements = append(replacements, replacement{start: position.Offset, end: fset.Position(literal.End()).Offset, text: errorName + ".Error()"})
						}
					}
				}
				call, ok := child.(*ast.CallExpr)
				if !ok {
					return true
				}
				literals := errorResponseMessageLiterals(call)
				if len(literals) == 0 {
					return true
				}
				for _, literal := range literals {
					position := fset.Position(literal.Pos())
					if seenResponses[position.Offset] {
						continue
					}
					seenResponses[position.Offset] = true
					end := fset.Position(literal.End())
					findings = append(findings, fmt.Sprintf("%s:%d: %s", file, position.Line, literal.Value))
					replacements = append(replacements, replacement{start: position.Offset, end: end.Offset, text: errorName + ".Error()"})
				}
				return true
			})
			return true
		})
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil || function.Type.Params == nil {
				continue
			}
			errorName := ""
			for _, field := range function.Type.Params.List {
				typeName, ok := field.Type.(*ast.Ident)
				if !ok || typeName.Name != "error" {
					continue
				}
				for _, name := range field.Names {
					if strings.Contains(strings.ToLower(name.Name), "err") || strings.Contains(strings.ToLower(name.Name), "cause") {
						errorName = name.Name
						break
					}
				}
				if errorName != "" {
					break
				}
			}
			if errorName == "" {
				continue
			}
			ast.Inspect(function.Body, func(child ast.Node) bool {
				call, ok := child.(*ast.CallExpr)
				if !ok {
					return true
				}
				literals := errorResponseMessageLiterals(call)
				if len(literals) == 0 {
					return true
				}
				for _, literal := range literals {
					position := fset.Position(literal.Pos())
					if seenResponses[position.Offset] {
						continue
					}
					seenResponses[position.Offset] = true
					end := fset.Position(literal.End())
					findings = append(findings, fmt.Sprintf("%s:%d: %s", file, position.Line, literal.Value))
					replacements = append(replacements, replacement{start: position.Offset, end: end.Offset, text: errorName + ".Error()"})
				}
				return true
			})
		}
		if mode != "fix" {
			continue
		}
		sort.Slice(replacements, func(i, j int) bool { return replacements[i].start > replacements[j].start })
		next := append([]byte(nil), source...)
		for _, item := range replacements {
			next = append(next[:item.start], append([]byte(item.text), next[item.end:]...)...)
		}
		if needsLog {
			var importErr error
			next, importErr = ensureLogImport(next, logName)
			if importErr != nil {
				panic(importErr)
			}
		}
		formatted, formatErr := formatSource(next)
		if formatErr != nil {
			panic(formatErr)
		}
		if !bytes.Equal(source, formatted) {
			if writeErr := os.WriteFile(file, formatted, 0644); writeErr != nil {
				panic(writeErr)
			}
			changed++
		}
	}
	if mode == "fix" {
		fmt.Printf("updated files: %d\n", changed)
	}
	if len(findings) > 0 {
		fmt.Println(strings.Join(findings, "\n"))
		os.Exit(1)
	}
	fmt.Println("generic backend error responses: 0")
}

func existingLogImportName(parsed *ast.File) string {
	for _, item := range parsed.Imports {
		path, err := strconv.Unquote(item.Path.Value)
		if err != nil || path != "log" {
			continue
		}
		if item.Name != nil {
			return item.Name.Name
		}
		return "log"
	}
	return ""
}

func ensureLogImport(source []byte, logName string) ([]byte, error) {
	if logName != "" && logName != "amitialog" {
		return source, nil
	}
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "", source, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	if existingLogImportName(parsed) != "" {
		return source, nil
	}
	spec := &ast.ImportSpec{Name: ast.NewIdent("amitialog"), Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote("log")}}
	var importDecl *ast.GenDecl
	for _, declaration := range parsed.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if ok && group.Tok == token.IMPORT {
			importDecl = group
			break
		}
	}
	if importDecl == nil {
		importDecl = &ast.GenDecl{Tok: token.IMPORT, Specs: []ast.Spec{spec}}
		parsed.Decls = append([]ast.Decl{importDecl}, parsed.Decls...)
	} else {
		importDecl.Specs = append(importDecl.Specs, spec)
		importDecl.Lparen = importDecl.TokPos
	}
	var output bytes.Buffer
	if err := format.Node(&output, fset, parsed); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func branchErrorName(statement *ast.IfStmt) string {
	names := map[string]bool{}
	if assignment, ok := statement.Init.(*ast.AssignStmt); ok {
		for index, left := range assignment.Lhs {
			identifier, isIdentifier := left.(*ast.Ident)
			if !isIdentifier || index >= len(assignment.Rhs) {
				continue
			}
			if !strings.Contains(strings.ToLower(identifier.Name), "err") {
				continue
			}
			if _, isCall := assignment.Rhs[index].(*ast.CallExpr); isCall {
				names[identifier.Name] = true
			}
		}
	}
	ast.Inspect(statement.Cond, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		lower := strings.ToLower(identifier.Name)
		if lower == "err" || strings.HasSuffix(lower, "err") || strings.HasPrefix(lower, "err") && lower != "errors" {
			names[identifier.Name] = true
		}
		return true
	})
	return errorConditionName(statement.Cond, names)
}

func errorConditionName(expression ast.Expr, names map[string]bool) string {
	switch value := expression.(type) {
	case *ast.BinaryExpr:
		if value.Op == token.NEQ {
			if identifier, ok := value.X.(*ast.Ident); ok && names[identifier.Name] && isNil(value.Y) {
				return identifier.Name
			}
			if identifier, ok := value.Y.(*ast.Ident); ok && names[identifier.Name] && isNil(value.X) {
				return identifier.Name
			}
		}
		if value.Op == token.LAND {
			if found := errorConditionName(value.X, names); found != "" {
				return found
			}
			return errorConditionName(value.Y, names)
		}
		if value.Op == token.LOR {
			left := errorConditionName(value.X, names)
			right := errorConditionName(value.Y, names)
			if left != "" && right != "" {
				return left
			}
			return ""
		}
	case *ast.CallExpr:
		selector, ok := value.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Is" || len(value.Args) == 0 {
			return ""
		}
		identifier, ok := value.Args[0].(*ast.Ident)
		if ok && names[identifier.Name] {
			return identifier.Name
		}
	case *ast.ParenExpr:
		return errorConditionName(value.X, names)
	case *ast.UnaryExpr:
		return errorConditionName(value.X, names)
	}
	return ""
}

func isNil(expression ast.Expr) bool {
	identifier, ok := expression.(*ast.Ident)
	return ok && identifier.Name == "nil"
}

func errorResponseMessageLiterals(call *ast.CallExpr) []*ast.BasicLit {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok {
		return nil
	}
	if selector.Sel.Name == "ErrorResponse" && (pkg.Name == "util" || pkg.Name == "response") && len(call.Args) >= 3 {
		literal, _ := call.Args[2].(*ast.BasicLit)
		if literal != nil && literal.Kind == token.STRING {
			return []*ast.BasicLit{literal}
		}
	}
	if pkg.Name != "c" || len(call.Args) < 2 {
		return nil
	}
	compositeIndex := -1
	switch selector.Sel.Name {
	case "JSON":
		if len(call.Args) >= 2 && !successStatus(call.Args[0]) {
			compositeIndex = 1
		}
	case "AbortWithStatusJSON":
		if len(call.Args) >= 3 && !successStatus(call.Args[0]) {
			compositeIndex = 2
		}
	}
	if compositeIndex < 0 {
		return nil
	}
	composite, ok := call.Args[compositeIndex].(*ast.CompositeLit)
	if !ok {
		return nil
	}
	humanKeys := map[string]bool{}
	for _, element := range composite.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := pair.Key.(*ast.BasicLit)
		if !ok || key.Kind != token.STRING {
			continue
		}
		name, err := strconv.Unquote(key.Value)
		if err != nil {
			continue
		}
		if name == "message" || name == "msg" {
			humanKeys[name] = true
		}
	}
	literals := []*ast.BasicLit{}
	for _, element := range composite.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := pair.Key.(*ast.BasicLit)
		if !ok || key.Kind != token.STRING {
			continue
		}
		name, err := strconv.Unquote(key.Value)
		if err != nil {
			continue
		}
		if len(humanKeys) > 0 {
			if !humanKeys[name] {
				continue
			}
		} else if name != "error" {
			continue
		}
		value, _ := pair.Value.(*ast.BasicLit)
		if value == nil || value.Kind != token.STRING {
			continue
		}
		text, err := strconv.Unquote(value.Value)
		if err != nil {
			continue
		}
		lower := strings.ToLower(text)
		if strings.Contains(lower, "fail") || strings.Contains(lower, "error") || strings.Contains(lower, "invalid") || strings.Contains(lower, "not found") || strings.Contains(lower, "stale") || strings.Contains(lower, "unavailable") || strings.Contains(text, "失败") || strings.Contains(text, "错误") || strings.Contains(text, "异常") || strings.Contains(text, "不可用") || strings.Contains(text, "无效") || strings.Contains(text, "超时") {
			literals = append(literals, value)
		}
	}
	return literals
}

func structuredErrorMessageLiterals(composite *ast.CompositeLit) []*ast.BasicLit {
	literals := []*ast.BasicLit{}
	for _, element := range composite.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := pair.Key.(*ast.Ident)
		if !ok || key.Name != "Message" && key.Name != "Msg" && key.Name != "message" && key.Name != "msg" {
			continue
		}
		value, _ := pair.Value.(*ast.BasicLit)
		if value == nil || value.Kind != token.STRING {
			continue
		}
		text, err := strconv.Unquote(value.Value)
		if err != nil {
			continue
		}
		lower := strings.ToLower(text)
		if strings.Contains(lower, "fail") || strings.Contains(lower, "error") || strings.Contains(lower, "invalid") || strings.Contains(lower, "not found") || strings.Contains(lower, "stale") || strings.Contains(lower, "unavailable") || strings.Contains(text, "失败") || strings.Contains(text, "错误") || strings.Contains(text, "异常") || strings.Contains(text, "不可用") || strings.Contains(text, "无效") || strings.Contains(text, "超时") {
			literals = append(literals, value)
		}
	}
	return literals
}

func successStatus(expression ast.Expr) bool {
	if selector, ok := expression.(*ast.SelectorExpr); ok {
		if pkg, ok := selector.X.(*ast.Ident); ok && pkg.Name == "http" {
			return selector.Sel.Name == "StatusOK" || selector.Sel.Name == "StatusCreated" || selector.Sel.Name == "StatusNoContent"
		}
	}
	value, ok := expression.(*ast.BasicLit)
	if !ok || value.Kind != token.INT {
		return false
	}
	return value.Value == "200" || value.Value == "201" || value.Value == "204"
}

func formatSource(source []byte) ([]byte, error) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "", source, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	used := false
	ast.Inspect(parsed, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		identifier, ok := selector.X.(*ast.Ident)
		if ok && identifier.Name == "amitialog" {
			used = true
		}
		return true
	})
	if !used {
		declarations := parsed.Decls[:0]
		for _, declaration := range parsed.Decls {
			group, ok := declaration.(*ast.GenDecl)
			if !ok || group.Tok != token.IMPORT {
				declarations = append(declarations, declaration)
				continue
			}
			specs := group.Specs[:0]
			for _, spec := range group.Specs {
				item, ok := spec.(*ast.ImportSpec)
				if ok && item.Name != nil && item.Name.Name == "amitialog" {
					continue
				}
				specs = append(specs, spec)
			}
			group.Specs = specs
			if len(group.Specs) > 0 {
				declarations = append(declarations, declaration)
			}
		}
		parsed.Decls = declarations
	}
	var output bytes.Buffer
	if err := format.Node(&output, fset, parsed); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
