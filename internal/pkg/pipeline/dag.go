package pipeline

import (
	"strings"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/dag/parser"
)

type dagExpr struct {
	expr parser.Expr
}

func (dx *dagExpr) IsEmpty() bool {
	return dx.expr == nil
}

func (dx dagExpr) String() string {
	if dx.expr == nil {
		return ""
	}
	return strings.Join(dx.expr.PostOrder(), " ")
}

func (d *dagExpr) UnmarshalYAML(unmarshal func(any) error) error {
	var expr string

	if err := unmarshal(&expr); err != nil {
		return err
	}

	parsedExpr, err := parser.ParseDAG(expr)
	if err != nil {
		return err
	}
	d.expr = parsedExpr

	return nil
}
