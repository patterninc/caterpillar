package xpath

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/antchfx/htmlquery"
	antchxpath "github.com/antchfx/xpath"
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

var compiledExprCache sync.Map

type matchedNode struct {
	owner  *html.Node
	output *html.Node
}

// antchfx/xpath evaluates "|" unions branch-by-branch rather than sorting into XPath 1.0 document order.
func findNodes(root *html.Node, expr string) []*html.Node {
	if !strings.Contains(expr, "|") {
		return htmlquery.Find(root, expr)
	}

	compiled, err := compileXPath(expr)
	if err != nil {
		panic(err)
	}

	t := compiled.Select(htmlquery.CreateXPathNavigator(root))
	var matches []matchedNode
	for t.MoveNext() {
		nav := t.Current().(*htmlquery.NodeNavigator)
		owner := nav.Current()
		output := owner
		if nav.NodeType() == antchxpath.AttributeNode {
			childNode := &html.Node{
				Type: html.TextNode,
				Data: nav.Value(),
			}
			output = &html.Node{
				Type:       html.ElementNode,
				Data:       nav.LocalName(),
				FirstChild: childNode,
				LastChild:  childNode,
			}
		}
		matches = append(matches, matchedNode{owner: owner, output: output})
	}

	return sortMatchesInDocumentOrder(root, matches)
}

func compileXPath(expr string) (*antchxpath.Expr, error) {
	if cached, ok := compiledExprCache.Load(expr); ok {
		return cached.(*antchxpath.Expr), nil
	}
	compiled, err := antchxpath.Compile(expr)
	if err != nil {
		return nil, err
	}
	compiledExprCache.Store(expr, compiled)
	return compiled, nil
}

func sortMatchesInDocumentOrder(root *html.Node, matches []matchedNode) []*html.Node {
	if len(matches) == 0 {
		return nil
	}
	if len(matches) == 1 {
		return []*html.Node{matches[0].output}
	}

	single := make(map[*html.Node]*html.Node, len(matches))
	var multi map[*html.Node][]*html.Node
	for _, m := range matches {
		if first, exists := single[m.owner]; !exists {
			single[m.owner] = m.output
		} else if first != nil {
			if multi == nil {
				multi = make(map[*html.Node][]*html.Node)
			}
			multi[m.owner] = []*html.Node{first, m.output}
			single[m.owner] = nil
		} else {
			multi[m.owner] = append(multi[m.owner], m.output)
		}
	}

	ordered := make([]*html.Node, 0, len(matches))
	remaining := len(single)

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if out, ok := single[n]; ok {
			if out != nil {
				ordered = append(ordered, out)
			} else {
				outs := multi[n]
				var self []*html.Node
				var attrs []*html.Node
				for _, o := range outs {
					if o == n {
						self = append(self, o)
					} else {
						attrs = append(attrs, o)
					}
				}
				if len(attrs) > 1 && len(n.Attr) > 1 {
					sortedAttrs := make([]*html.Node, 0, len(attrs))
					for _, a := range n.Attr {
						for _, attrNode := range attrs {
							if attrNode.Data == a.Key && attrNode.FirstChild != nil && attrNode.FirstChild.Data == a.Val {
								sortedAttrs = append(sortedAttrs, attrNode)
							}
						}
					}
					if len(sortedAttrs) == len(attrs) {
						attrs = sortedAttrs
					}
				}
				ordered = append(ordered, self...)
				ordered = append(ordered, attrs...)
			}
			delete(single, n)
			remaining--
			if remaining == 0 {
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
			if remaining == 0 {
				return
			}
		}
	}
	walk(root)

	if remaining > 0 && root.Parent != nil {
		top := root
		for top.Parent != nil {
			top = top.Parent
		}
		clear(single)
		clear(multi)
		for _, m := range matches {
			if first, exists := single[m.owner]; !exists {
				single[m.owner] = m.output
			} else if first != nil {
				if multi == nil {
					multi = make(map[*html.Node][]*html.Node)
				}
				multi[m.owner] = []*html.Node{first, m.output}
				single[m.owner] = nil
			} else {
				multi[m.owner] = append(multi[m.owner], m.output)
			}
		}
		ordered = ordered[:0]
		remaining = len(single)
		walk(top)
	}

	return ordered
}

