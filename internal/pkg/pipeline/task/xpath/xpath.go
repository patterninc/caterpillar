package xpath

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"

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
			return err
		}

		containerNodes := []*html.Node{document}
		if x.Container != `` {
			containerNodes = htmlquery.Find(document, x.Container)
			if len(containerNodes) == 0 {
				return fmt.Errorf("no nodes found for XPath: %s", x.Container)
			}
		}

		for i, container := range containerNodes {
			data, err := x.queryFields(container)
			if err != nil {
				return err
			}

			if len(data) != 0 {
				index := fmt.Sprintf("%d", i+1)
				r.SetContextValue(nodeIndexKey, index)
				x.SendData(r.Context, data, output)
			}
		}
	}

	return nil

}

func (x *xpath) queryFields(container *html.Node) ([]byte, error) {

	result := make(map[string]any)

	for field, xpathExpr := range x.Fields {
		nodes := htmlquery.Find(container, xpathExpr)

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
