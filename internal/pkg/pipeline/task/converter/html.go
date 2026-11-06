package converter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/antchfx/htmlquery"
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
		containerNodes = htmlquery.Find(document, c.Container)
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
