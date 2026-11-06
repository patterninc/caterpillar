package xpath

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/converter"
)

const nodeIndexKey = "node_index"

type xpath struct {
	task.Base     `yaml:",inline" json:",inline"`
	Container     string            `yaml:"container" json:"container"`
	Fields        map[string]string `yaml:"fields" json:"fields"`
	IgnoreMissing bool              `yaml:"ignore_missing" json:"ignore_missing"` // if true, missing fields will not cause an error
}

func New() (task.Task, error) {
	return &xpath{IgnoreMissing: true}, nil
}

func (x *xpath) Run(input <-chan *record.Record, output chan<- *record.Record) error {

	for {
		r, ok := x.GetRecord(input)
		if !ok {
			break
		}

		document, err := htmlquery.Parse(bytes.NewReader(r.Data))
		if err != nil {
			return ack.Rejected(r.Context, err)
		}

		containerNodes := []*html.Node{document}
		if x.Container != `` {
			containerNodes = findNodes(document, x.Container)
			if len(containerNodes) == 0 {
				if !x.IgnoreMissing {
					return ack.Rejected(r.Context, fmt.Errorf("no nodes found for XPath: %s", x.Container))
				}
				fmt.Println("container is missing - ", x.Container)
				ack.Release(r.Context)
				continue
			}
		}

		for i, container := range containerNodes {
			data, err := x.queryFields(container)
			if err != nil {
				return ack.Rejected(r.Context, err)
			}

			if len(data) != 0 {
				index := fmt.Sprintf("%d", i+1)
				r.SetContextValue(nodeIndexKey, index)
				x.SendData(r.Context, data, output)
			}
		}

		ack.Release(r.Context)
	}

	return nil

}

func (x *xpath) queryFields(container *html.Node) ([]byte, error) {

	result := make(map[string]any)

	for field, xpathExpr := range x.Fields {
		nodes := findNodes(container, xpathExpr)

		if len(nodes) == 0 && !x.IgnoreMissing {
			return nil, fmt.Errorf("field '%s' not found for xpath: %s", field, xpathExpr)
		}

		if len(nodes) == 0 {
			result[field] = nil
			continue
		}

		values := make([]any, 0, len(nodes))
		for _, node := range nodes {
			if isNonLeaf(node) || isEmpty(node) {
				values = append(values, converter.ConvertHtmlNode(node))
			} else {
				values = append(values, strings.TrimSpace(htmlquery.InnerText(node)))
			}
		}
		result[field] = values
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %v", err)
	}

	return jsonData, nil

}

func isNonLeaf(node *html.Node) bool {
	return node.FirstChild != nil && node.FirstChild.NextSibling != nil
}

func isEmpty(node *html.Node) bool {
	return node.FirstChild == nil
}

// antchfx/xpath evaluates "|" unions branch-by-branch rather than sorting into XPath 1.0 document order.
func findNodes(root *html.Node, expr string) []*html.Node {
	nodes := htmlquery.Find(root, expr)
	if len(nodes) > 1 && strings.Contains(expr, "|") {
		return sortNodesInDocumentOrder(root, nodes)
	}
	return nodes
}

func sortNodesInDocumentOrder(root *html.Node, nodes []*html.Node) []*html.Node {
	if len(nodes) <= 1 {
		return nodes
	}

	selected := make(map[*html.Node]struct{}, len(nodes))
	for _, n := range nodes {
		selected[n] = struct{}{}
	}

	ordered := make([]*html.Node, 0, len(nodes))
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if _, ok := selected[n]; ok {
			ordered = append(ordered, n)
			delete(selected, n)
			if len(selected) == 0 {
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
			if len(selected) == 0 {
				return
			}
		}
	}
	walk(root)

	if len(selected) > 0 && root.Parent != nil {
		hasAttachedNode := false
		for n := range selected {
			if n.Parent != nil {
				hasAttachedNode = true
				break
			}
		}
		if hasAttachedNode {
			top := root
			for top.Parent != nil {
				top = top.Parent
			}
			for _, n := range ordered {
				selected[n] = struct{}{}
			}
			ordered = ordered[:0]
			walk(top)
		}
	}

	// Synthetic attribute nodes returned by htmlquery have no parent in the DOM tree.
	if len(selected) > 0 {
		for _, n := range nodes {
			if _, ok := selected[n]; ok {
				ordered = append(ordered, n)
			}
		}
	}

	return ordered
}

