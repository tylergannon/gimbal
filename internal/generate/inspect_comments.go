package generate

import (
	"go/ast"
	"strings"
)

type diagramAnnotation struct {
	owner, fallback    controlKey
	title, description string
	trueLabel          string
}

// Plain comments immediately above a statement describe that statement. Go's
// comment map excludes trailing comments owned by the previous statement; the
// adjacency check excludes detached prose. Function bodies are separate owners.
func (e *extractor) inspectDiagramComments() {
	for _, file := range e.pkg.Syntax {
		comments := ast.NewCommentMap(e.pkg.Fset, file, file.Comments)
		ast.Inspect(file, func(node ast.Node) bool {
			stmt, ok := node.(ast.Stmt)
			if !ok {
				return true
			}
			for _, group := range comments[stmt] {
				if e.pkg.Fset.Position(group.End()).Line+1 != e.pkg.Fset.Position(stmt.Pos()).Line {
					continue
				}
				lines := strings.Split(strings.TrimSpace(group.Text()), "\n")
				if lines[0] == "" {
					continue
				}
				a := diagramAnnotation{title: lines[0]}
				var description []string
				_, isIf := stmt.(*ast.IfStmt)
				for _, line := range lines[1:] {
					if value, ok := strings.CutPrefix(line, "When true:"); ok && isIf {
						a.trueLabel = strings.TrimSpace(value)
					} else {
						description = append(description, line)
					}
				}
				a.description = strings.TrimSpace(strings.Join(description, "\n"))
				switch stmt.(type) {
				case *ast.CaseClause:
					a.owner = controlKey{Source: e.at(stmt.Pos()), Kind: "Branch"}
				case *ast.IfStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt:
					a.owner = controlKey{Source: e.at(stmt.Pos()), Kind: "Condition"}
				case *ast.ForStmt, *ast.RangeStmt:
					a.owner = controlKey{Source: e.at(stmt.Pos()), Kind: "Repeat"}
				}
				ast.Inspect(stmt, func(n ast.Node) bool {
					if _, ok := n.(*ast.FuncLit); ok {
						return false
					}
					if _, ok := n.(*ast.BlockStmt); ok {
						return false
					}
					call, ok := n.(*ast.CallExpr)
					if !ok || a.fallback.Kind != "" {
						return a.fallback.Kind == ""
					}
					kind, ok := e.gimbalCall(call)
					if !ok {
						return true
					}
					switch kind {
					case "RunCommand", "Check":
						kind = "Command"
					case "NewSession", "Fork", "Generate", "Interview", "Service", "Set", "SetJSON", "Scope", "Group", "Go", "PromiseLoop", "Iterate":
					default:
						return true
					}
					a.fallback = controlKey{Source: e.at(call.Pos()), Kind: kind}
					return false
				})
				e.inspection.Annotations = append(e.inspection.Annotations, a)
			}
			return true
		})
	}
}

func applyDiagramAnnotations(nodes []*viewNode, annotations []diagramAnnotation) {
	bySource := map[controlKey][]*viewNode{}
	branchesBySource := map[controlKey][]*branchInspection{}
	var index func([]*viewNode)
	index = func(nodes []*viewNode) {
		for _, n := range nodes {
			key := controlKey{Source: n.Source, Kind: n.Kind}
			bySource[key] = append(bySource[key], n)
			for i := range n.Branches {
				br := &n.Branches[i]
				key := controlKey{Source: br.Source, Kind: "Branch"}
				branchesBySource[key] = append(branchesBySource[key], br)
			}
			for _, child := range n.Children {
				index(child)
			}
		}
	}
	index(nodes)
	for _, a := range annotations {
		if a.owner.Kind == "Condition" || a.owner.Kind == "Branch" {
			for _, br := range branchesBySource[controlKey{Source: a.owner.Source, Kind: "Branch"}] {
				br.Title, br.Description, br.Label = a.title, a.description, a.trueLabel
			}
		}
		if a.owner.Kind == "Branch" {
			continue
		}
		owners := bySource[a.owner]
		// An omitted error-handling guard lends its comment to its initializer
		// operation. A real condition keeps the comment, never duplicating it.
		if len(owners) == 0 {
			owners = bySource[a.fallback]
		}
		for _, n := range owners {
			n.Title, n.Description = a.title, a.description
		}
	}
}
