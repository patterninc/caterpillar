package converter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/antchfx/htmlquery"
	antchxpath "github.com/antchfx/xpath"
	h "golang.org/x/net/html"
)

type html struct {
	Container string `yaml:"container,omitempty" json:"container,omitempty"`
}

type htmlElement struct {
	Tag        string            `json:"tag"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Text       string            `json:"text,omitempty"`
	Children   []*htmlElement    `json:"children,omitempty"`
}

func (c *html) convert(data []byte, _ string) ([]converterOutput, error) {

	// Parse the HTML content using htmlquery
	document, err := htmlquery.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %v", err)
	}

	// we'll start with the input based on the whole document and...
	// adjust to the container if requested...
	containerNodes := []*h.Node{document}
	if c.Container != `` {
		containerNodes = FindNodes(document, c.Container)
		if len(containerNodes) == 0 {
			return nil, fmt.Errorf("no nodes found for XPath: %s", c.Container)
		}
	}

	// now let's work through the nodes we got...
	elements := make([]*htmlElement, 0, len(containerNodes))
	for _, node := range containerNodes {
		elements = append(elements, ConvertHtmlNode(node))
	}

	jsonData, err := json.Marshal(elements)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %v", err)
	}

	return []converterOutput{{Data: jsonData}}, nil

}

func ConvertHtmlNode(node *h.Node) *htmlElement {

	element := &htmlElement{
		Tag: node.Data,
	}

	// Extract attributes
	if len(node.Attr) > 0 {
		element.Attributes = make(map[string]string)
		for _, attr := range node.Attr {
			element.Attributes[attr.Key] = attr.Val
		}
	}

	// Extract direct text content (excluding child nodes)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == h.TextNode {
			element.Text = element.Text + strings.TrimSpace(child.Data)
		}
	}

	// Process child nodes
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == h.ElementNode {
			element.Children = append(element.Children, ConvertHtmlNode(child))
		}
	}

	return element

}

// CompiledXPath holds a pre-compiled XPath expression and whether it contains a union ("|") operator.
type CompiledXPath struct {
	expr    *antchxpath.Expr
	isUnion bool
}

type matchedNode struct {
	owner  *h.Node
	output *h.Node
}

// CompileXPath compiles an XPath expression and detects whether it uses the union ("|") operator outside string literals.
func CompileXPath(expr string) (CompiledXPath, error) {
	compiled, err := antchxpath.Compile(expr)
	if err != nil {
		return CompiledXPath{}, err
	}
	return CompiledXPath{
		expr:    compiled,
		isUnion: hasUnionOperator(expr),
	}, nil
}

// Find executes the compiled XPath against root, sorting results into XPath 1.0 document order for union queries.
func (cx CompiledXPath) Find(root *h.Node) []*h.Node {
	if !cx.isUnion {
		return htmlquery.QuerySelectorAll(root, cx.expr)
	}

	t := cx.expr.Select(htmlquery.CreateXPathNavigator(root))
	var matches []matchedNode
	for t.MoveNext() {
		nav := t.Current().(*htmlquery.NodeNavigator)
		owner := nav.Current()
		output := owner
		if nav.NodeType() == antchxpath.AttributeNode {
			childNode := &h.Node{
				Type: h.TextNode,
				Data: nav.Value(),
			}
			output = &h.Node{
				Type:       h.ElementNode,
				Data:       nav.LocalName(),
				FirstChild: childNode,
				LastChild:  childNode,
			}
		}
		matches = append(matches, matchedNode{owner: owner, output: output})
	}

	return sortMatchesInDocumentOrder(root, matches)
}

// FindNodes evaluates expr against root and returns matched nodes in XPath 1.0 document order.
func FindNodes(root *h.Node, expr string) []*h.Node {
	if !hasUnionOperator(expr) {
		return htmlquery.Find(root, expr)
	}
	cx, err := CompileXPath(expr)
	if err != nil {
		panic(err)
	}
	return cx.Find(root)
}

func hasUnionOperator(expr string) bool {
	var quote byte
	for i := 0; i < len(expr); i++ {
		ch := expr[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '\'', '"':
			quote = ch
		case '|':
			return true
		}
	}
	return false
}

func sortMatchesInDocumentOrder(root *h.Node, matches []matchedNode) []*h.Node {
	if len(matches) == 0 {
		return nil
	}
	if len(matches) == 1 {
		return []*h.Node{matches[0].output}
	}

	single := make(map[*h.Node]*h.Node, len(matches))
	var multi map[*h.Node][]*h.Node
	populate := func() {
		clear(single)
		clear(multi)
		for _, m := range matches {
			if first, exists := single[m.owner]; !exists {
				single[m.owner] = m.output
			} else if first != nil {
				if multi == nil {
					multi = make(map[*h.Node][]*h.Node)
				}
				multi[m.owner] = []*h.Node{first, m.output}
				single[m.owner] = nil
			} else {
				multi[m.owner] = append(multi[m.owner], m.output)
			}
		}
	}
	populate()

	ordered := make([]*h.Node, 0, len(matches))
	remaining := len(single)

	var walk func(*h.Node)
	walk = func(n *h.Node) {
		if out, ok := single[n]; ok {
			if out != nil {
				ordered = append(ordered, out)
			} else {
				outs := multi[n]
				var self []*h.Node
				var attrs []*h.Node
				for _, o := range outs {
					if o == n {
						self = append(self, o)
					} else {
						attrs = append(attrs, o)
					}
				}
				if len(attrs) > 1 && len(n.Attr) > 1 {
					sortedAttrs := make([]*h.Node, 0, len(attrs))
					for _, a := range n.Attr {
						for i, attrNode := range attrs {
							if attrNode != nil && attrNode.Data == a.Key && attrNode.FirstChild != nil && attrNode.FirstChild.Data == a.Val {
								sortedAttrs = append(sortedAttrs, attrNode)
								attrs[i] = nil
								break
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
		populate()
		ordered = ordered[:0]
		remaining = len(single)
		walk(top)
	}

	return ordered
}
