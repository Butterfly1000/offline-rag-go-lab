package documentingest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

func parseGoSource(sourceRef, content string) ([]structuralUnit, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, sourceRef, content, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return nil, fmt.Errorf("parse Go source: %w", err)
	}
	packagePath := "package " + file.Name.Name
	var units []structuralUnit

	firstDeclarationOffset := len(content)
	if len(file.Decls) > 0 {
		firstDeclarationOffset = nodeStartOffset(fileSet, file.Decls[0])
		if doc := declarationDoc(file.Decls[0]); doc != nil {
			firstDeclarationOffset = fileSet.PositionFor(doc.Pos(), false).Offset
		}
	}
	if preamble := normalizePortableText(content[:firstDeclarationOffset]); preamble != "" {
		units = append(units, structuralUnit{
			Kind: "go_preamble", HeadingPath: packagePath, Text: preamble,
		})
	}

	previousEnd := firstDeclarationOffset
	for _, declaration := range file.Decls {
		start := nodeStartOffset(fileSet, declaration)
		if doc := declarationDoc(declaration); doc != nil {
			start = fileSet.PositionFor(doc.Pos(), false).Offset
		}
		// AST declaration ranges exclude detached comments. Attach any non-space
		// interstitial source to the following declaration so ingestion never
		// silently drops operational notes that are not Go doc comments.
		if previousEnd < start && strings.TrimSpace(content[previousEnd:start]) != "" {
			start = previousEnd
		}
		end := fileSet.PositionFor(declaration.End(), false).Offset
		if start < 0 || end < start || end > len(content) {
			return nil, fmt.Errorf("Go declaration offsets %d:%d exceed source length %d", start, end, len(content))
		}
		text := normalizePortableText(content[start:end])
		if text == "" {
			continue
		}
		units = append(units, structuralUnit{
			Kind: "go_declaration", HeadingPath: packagePath + " / " + declarationName(declaration), Text: text,
		})
		previousEnd = end
	}
	if len(file.Decls) > 0 && previousEnd < len(content) && strings.TrimSpace(content[previousEnd:]) != "" {
		last := &units[len(units)-1]
		last.Text = normalizePortableText(last.Text + content[previousEnd:])
	}
	return units, nil
}

func nodeStartOffset(fileSet *token.FileSet, node ast.Node) int {
	return fileSet.PositionFor(node.Pos(), false).Offset
}

func declarationDoc(declaration ast.Decl) *ast.CommentGroup {
	switch value := declaration.(type) {
	case *ast.FuncDecl:
		return value.Doc
	case *ast.GenDecl:
		return value.Doc
	default:
		return nil
	}
}

func declarationName(declaration ast.Decl) string {
	switch value := declaration.(type) {
	case *ast.FuncDecl:
		name := value.Name.Name
		if value.Recv != nil && len(value.Recv.List) > 0 {
			if receiver := goTypeName(value.Recv.List[0].Type); receiver != "" {
				name = receiver + "." + name
			}
		}
		return "func " + name
	case *ast.GenDecl:
		if len(value.Specs) == 0 {
			return value.Tok.String()
		}
		switch spec := value.Specs[0].(type) {
		case *ast.TypeSpec:
			return "type " + spec.Name.Name
		case *ast.ValueSpec:
			if len(spec.Names) > 0 {
				return value.Tok.String() + " " + spec.Names[0].Name
			}
		case *ast.ImportSpec:
			return "import"
		}
		return value.Tok.String()
	default:
		return "declaration"
	}
}

func goTypeName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		return goTypeName(value.X)
	case *ast.IndexExpr:
		return goTypeName(value.X)
	case *ast.IndexListExpr:
		return goTypeName(value.X)
	case *ast.SelectorExpr:
		prefix := goTypeName(value.X)
		return strings.TrimPrefix(prefix+"."+value.Sel.Name, ".")
	default:
		return ""
	}
}
