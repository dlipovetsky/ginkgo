package outline

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"strings"

	"golang.org/x/tools/go/ast/inspector"
)

const (
	// ginkgoImportPath is the well-known ginkgo import path
	ginkgoImportPath = "github.com/onsi/ginkgo"
)

// FromASTFile returns an outline for a Ginkgo test source file
func FromASTFile(src *ast.File) (*outline, error) {
	ginkgoImportName, ok := importNameForPackage(src, ginkgoImportPath)
	if !ok {
		return nil, fmt.Errorf("file does not import %s", ginkgoImportPath)
	}
	root := ginkgoNode{
		Nodes: []*ginkgoNode{},
	}
	stack := []*ginkgoNode{&root}

	ispr := inspector.New([]*ast.File{src})
	ispr.Nodes([]ast.Node{(*ast.CallExpr)(nil)}, func(node ast.Node, push bool) bool {
		if push {
			// Pre-order traversal
			ce, ok := node.(*ast.CallExpr)
			if !ok {
				// Because `Nodes` calls this function only when the node is an
				// ast.CallExpr, this should never happen
				panic(fmt.Errorf("node starting at %d, ending at %d is not an *ast.CallExpr", node.Pos(), node.End()))
			}
			gn, ok := ginkgoNodeFromCallExpr(ce, ginkgoImportName)
			if !ok {
				// Node is not a Ginkgo spec or container, continue
				return true
			}

			parent := stack[len(stack)-1]
			parent.Nodes = append(parent.Nodes, gn)

			stack = append(stack, gn)
			return true
		}
		// Post-order traversal
		lastVisitedGinkgoNode := stack[len(stack)-1]
		if node.Pos() != lastVisitedGinkgoNode.Start || node.End() != lastVisitedGinkgoNode.End {
			// Node is not a Ginkgo spec or container, so it was not pushed onto the stack, continue
			return true
		}
		stack = stack[0 : len(stack)-1]
		return true
	})

	// Derive the final focused property for all nodes. This must be done
	// _before_ propagating the inherited focused property.
	root.BackpropagateUnfocus()
	// Now, propagate inherited properties, including focused and pending.
	root.PropagateInheritedProperties()

	return &outline{root}, nil
}

type outline struct {
	ginkgoNode
}

func (o *outline) MarshalJSON() ([]byte, error) {
	return json.Marshal(o.Nodes)
}

// String returns a CSV-formatted outline. Spec or container are output in
// depth-first order.
func (o *outline) String() string {
	var b strings.Builder
	b.WriteString("Name,Text,Start,End,Spec,Focused,Pending\n")
	f := func(n *ginkgoNode) {
		b.WriteString(fmt.Sprintf("%s,%s,%d,%d,%t,%t,%t\n", n.Name, n.Text, n.Start, n.End, n.Spec, n.Focused, n.Pending))
	}
	for _, n := range o.Nodes {
		n.PreOrder(f)
	}
	return b.String()
}
