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
		containerNodes, err = FindNodes(document, c.Container)
		if err != nil {
			return nil, fmt.Errorf("invalid XPath %q: %w", c.Container, err)
		}
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

// IsCompiled reports whether the XPath expression has been compiled.
func (cx CompiledXPath) IsCompiled() bool {
	return cx.expr != nil
}

// Find executes the compiled XPath against root, sorting results into XPath 1.0 document order for union queries.
func (cx CompiledXPath) Find(root *h.Node) []*h.Node {
	if !cx.isUnion {
		return htmlquery.QuerySelectorAll(root, cx.expr)
	}

	t := cx.expr.Select(htmlquery.CreateXPathNavigator(root))
	var matches []matchedNode
	for t.MoveNext() {
		matches = append(matches, newMatchedNode(t.Current().(*htmlquery.NodeNavigator)))
	}

	return sortMatchesInDocumentOrder(root, matches)
}

func newMatchedNode(nav *htmlquery.NodeNavigator) matchedNode {
	owner := nav.Current()
	if nav.NodeType() != antchxpath.AttributeNode {
		return matchedNode{owner: owner, output: owner}
	}
	childNode := &h.Node{
		Type: h.TextNode,
		Data: nav.Value(),
	}
	output := &h.Node{
		Type:       h.ElementNode,
		Data:       nav.LocalName(),
		FirstChild: childNode,
		LastChild:  childNode,
	}
	return matchedNode{owner: owner, output: output}
}

// FindNodes evaluates expr against root and returns matched nodes in XPath 1.0 document order.
func FindNodes(root *h.Node, expr string) ([]*h.Node, error) {
	if !hasUnionOperator(expr) {
		return htmlquery.QueryAll(root, expr)
	}
	cx, err := CompileXPath(expr)
	if err != nil {
		return nil, err
	}
	return cx.Find(root), nil
}

func hasUnionOperator(expr string) bool {
	var quote byte
	for i := 0; i < len(expr); i++ {
		ch := expr[i]
		if quote != 0 {
			quote = updateQuoteState(quote, ch)
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			continue
		}
		if ch == '|' {
			return true
		}
	}
	return false
}

func updateQuoteState(quote, ch byte) byte {
	if ch == quote {
		return 0
	}
	return quote
}

type matchIndex struct {
	single    map[*h.Node]*h.Node
	multi     map[*h.Node][]*h.Node
	ordered   []*h.Node
	remaining int
}

func sortMatchesInDocumentOrder(root *h.Node, matches []matchedNode) []*h.Node {
	if len(matches) == 0 {
		return nil
	}
	if len(matches) == 1 {
		return []*h.Node{matches[0].output}
	}

	idx := &matchIndex{
		single:  make(map[*h.Node]*h.Node, len(matches)),
		ordered: make([]*h.Node, 0, len(matches)),
	}
	idx.populate(matches)
	idx.walk(root)

	if idx.remaining > 0 && root.Parent != nil {
		idx.populate(matches)
		idx.walk(topNode(root))
	}

	return idx.ordered
}

func (idx *matchIndex) populate(matches []matchedNode) {
	clear(idx.single)
	clear(idx.multi)
	for _, m := range matches {
		idx.add(m)
	}
	idx.ordered = idx.ordered[:0]
	idx.remaining = len(idx.single)
}

func (idx *matchIndex) add(m matchedNode) {
	first, exists := idx.single[m.owner]
	if !exists {
		idx.single[m.owner] = m.output
		return
	}
	if first != nil {
		if idx.multi == nil {
			idx.multi = make(map[*h.Node][]*h.Node)
		}
		idx.multi[m.owner] = []*h.Node{first, m.output}
		idx.single[m.owner] = nil
		return
	}
	idx.multi[m.owner] = append(idx.multi[m.owner], m.output)
}

func (idx *matchIndex) walk(n *h.Node) {
	if out, ok := idx.single[n]; ok {
		idx.collectOwner(n, out)
		if idx.remaining == 0 {
			return
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		idx.walk(c)
		if idx.remaining == 0 {
			return
		}
	}
}

func (idx *matchIndex) collectOwner(n, out *h.Node) {
	delete(idx.single, n)
	idx.remaining--
	if out != nil {
		idx.ordered = append(idx.ordered, out)
		return
	}

	var self, attrs []*h.Node
	for _, o := range idx.multi[n] {
		if o == n {
			self = append(self, o)
		} else {
			attrs = append(attrs, o)
		}
	}
	idx.ordered = append(idx.ordered, self...)
	idx.ordered = append(idx.ordered, orderOwnerAttrs(n, attrs)...)
}

func orderOwnerAttrs(n *h.Node, attrs []*h.Node) []*h.Node {
	if len(attrs) <= 1 || len(n.Attr) <= 1 {
		return attrs
	}
	sorted := make([]*h.Node, 0, len(attrs))
	for _, a := range n.Attr {
		if matched := takeMatchingAttr(attrs, a); matched != nil {
			sorted = append(sorted, matched)
		}
	}
	if len(sorted) != len(attrs) {
		return attrs
	}
	return sorted
}

func takeMatchingAttr(attrs []*h.Node, a h.Attribute) *h.Node {
	for i, attrNode := range attrs {
		if attrNode != nil && attrNode.Data == a.Key && attrNode.FirstChild != nil && attrNode.FirstChild.Data == a.Val {
			attrs[i] = nil
			return attrNode
		}
	}
	return nil
}

func topNode(n *h.Node) *h.Node {
	for n.Parent != nil {
		n = n.Parent
	}
	return n
}
