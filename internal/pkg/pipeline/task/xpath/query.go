package xpath

import (
	"github.com/antchfx/htmlquery"
	antchxpath "github.com/antchfx/xpath"
	"golang.org/x/net/html"
)

// compiledXPath holds a pre-compiled XPath expression and whether it uses the union ("|") operator.
type compiledXPath struct {
	expr    *antchxpath.Expr
	isUnion bool
}

type matchedNode struct {
	owner  *html.Node
	output *html.Node
}

// antchfx/xpath evaluates "|" unions branch-by-branch rather than sorting into XPath 1.0 document order.
func compileXPath(expr string) (compiledXPath, error) {
	compiled, err := antchxpath.Compile(expr)
	if err != nil {
		return compiledXPath{}, err
	}
	return compiledXPath{
		expr:    compiled,
		isUnion: hasUnionOperator(expr),
	}, nil
}

func (cx compiledXPath) find(root *html.Node) []*html.Node {
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
	childNode := &html.Node{
		Type: html.TextNode,
		Data: nav.Value(),
	}
	output := &html.Node{
		Type:       html.ElementNode,
		Data:       nav.LocalName(),
		FirstChild: childNode,
		LastChild:  childNode,
	}
	return matchedNode{owner: owner, output: output}
}

func hasUnionOperator(expr string) bool {
	var quote byte
	for i := 0; i < len(expr); i++ {
		ch := expr[i]
		switch {
		case quote != 0 && ch == quote:
			quote = 0
		case quote != 0:
		case ch == '\'' || ch == '"':
			quote = ch
		case ch == '|':
			return true
		}
	}
	return false
}

type matchIndex struct {
	single    map[*html.Node]*html.Node
	multi     map[*html.Node][]*html.Node
	ordered   []*html.Node
	remaining int
}

func sortMatchesInDocumentOrder(root *html.Node, matches []matchedNode) []*html.Node {
	if len(matches) == 0 {
		return nil
	}
	if len(matches) == 1 {
		return []*html.Node{matches[0].output}
	}

	idx := &matchIndex{
		single:  make(map[*html.Node]*html.Node, len(matches)),
		ordered: make([]*html.Node, 0, len(matches)),
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
			idx.multi = make(map[*html.Node][]*html.Node)
		}
		idx.multi[m.owner] = []*html.Node{first, m.output}
		idx.single[m.owner] = nil
		return
	}
	idx.multi[m.owner] = append(idx.multi[m.owner], m.output)
}

func (idx *matchIndex) walk(n *html.Node) {
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

func (idx *matchIndex) collectOwner(n, out *html.Node) {
	delete(idx.single, n)
	idx.remaining--
	if out != nil {
		idx.ordered = append(idx.ordered, out)
		return
	}

	var self, attrs []*html.Node
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

func orderOwnerAttrs(n *html.Node, attrs []*html.Node) []*html.Node {
	if len(attrs) <= 1 || len(n.Attr) <= 1 {
		return attrs
	}
	sorted := make([]*html.Node, 0, len(attrs))
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

func takeMatchingAttr(attrs []*html.Node, a html.Attribute) *html.Node {
	for i, attrNode := range attrs {
		if attrNode != nil && attrNode.Data == a.Key && attrNode.FirstChild != nil && attrNode.FirstChild.Data == a.Val {
			attrs[i] = nil
			return attrNode
		}
	}
	return nil
}

func topNode(n *html.Node) *html.Node {
	for n.Parent != nil {
		n = n.Parent
	}
	return n
}
