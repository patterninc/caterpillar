package xpath

import (
	"github.com/antchfx/htmlquery"
	antchxpath "github.com/antchfx/xpath"
	"golang.org/x/net/html"
)

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
	return compiledXPath{expr: compiled, isUnion: hasUnionOperator(expr)}, nil
}

func (cx compiledXPath) find(root *html.Node) []*html.Node {
	if !cx.isUnion {
		return htmlquery.QuerySelectorAll(root, cx.expr)
	}
	matches := collectMatches(root, cx.expr)
	if len(matches) <= 1 {
		return unwrapMatches(matches)
	}
	return orderByDOM(root, matches)
}

func collectMatches(root *html.Node, expr *antchxpath.Expr) []matchedNode {
	t := expr.Select(htmlquery.CreateXPathNavigator(root))
	var matches []matchedNode
	for t.MoveNext() {
		matches = append(matches, newMatchedNode(t.Current().(*htmlquery.NodeNavigator)))
	}
	return matches
}

func newMatchedNode(nav *htmlquery.NodeNavigator) matchedNode {
	owner := nav.Current()
	if nav.NodeType() != antchxpath.AttributeNode {
		return matchedNode{owner: owner, output: owner}
	}
	text := &html.Node{Type: html.TextNode, Data: nav.Value()}
	attr := &html.Node{Type: html.ElementNode, Data: nav.LocalName(), FirstChild: text, LastChild: text}
	return matchedNode{owner: owner, output: attr}
}

func unwrapMatches(matches []matchedNode) []*html.Node {
	if len(matches) == 0 {
		return nil
	}
	return []*html.Node{matches[0].output}
}

func orderByDOM(root *html.Node, matches []matchedNode) []*html.Node {
	ordered := walkDOM(root, groupByOwner(matches), len(matches))
	if len(ordered) < len(matches) && root.Parent != nil {
		return walkDOM(topNode(root), groupByOwner(matches), len(matches))
	}
	return ordered
}

func groupByOwner(matches []matchedNode) map[*html.Node][]*html.Node {
	byOwner := make(map[*html.Node][]*html.Node, len(matches))
	for _, m := range matches {
		byOwner[m.owner] = append(byOwner[m.owner], m.output)
	}
	return byOwner
}

func walkDOM(root *html.Node, byOwner map[*html.Node][]*html.Node, capacity int) []*html.Node {
	ordered := make([]*html.Node, 0, capacity)
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if outs, ok := byOwner[n]; ok {
			ordered = appendOwnerMatches(ordered, n, outs)
			delete(byOwner, n)
		}
		for c := n.FirstChild; c != nil && len(byOwner) > 0; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return ordered
}

// XPath 1.0 places an element before its own attributes in document order.
// shortcut: multiple attributes matched on the same element stay in branch order, sort against owner.Attr if needed.
func appendOwnerMatches(dst []*html.Node, owner *html.Node, outs []*html.Node) []*html.Node {
	for _, o := range outs {
		if o == owner {
			dst = append(dst, o)
		}
	}
	for _, o := range outs {
		if o != owner {
			dst = append(dst, o)
		}
	}
	return dst
}

func topNode(n *html.Node) *html.Node {
	for n.Parent != nil {
		n = n.Parent
	}
	return n
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
