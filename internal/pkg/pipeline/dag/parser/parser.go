package parser

import (
	"errors"
	"fmt"
)

type Parser struct {
	tokens []Token
	pos    int
}

func (p *Parser) current() Token {
	return p.tokens[p.pos]
}

func (p *Parser) eat(tt TokenType) bool {
	if p.current().Type == tt {
		p.pos++
		return true
	}
	return false
}

func (p *Parser) parseTerm() (Expr, error) {
	tok := p.current()
	if p.eat(TokenIdent) {
		return &Ident{Name: tok.Value}, nil
	}

	if p.eat(TokenLBracket) {
		exprs := make([]Expr, 0)

		for {
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			exprs = append(exprs, expr)

			if !p.eat(TokenComma) || p.current().Type == TokenRBracket {
				break
			}
		}

		if !p.eat(TokenRBracket) {
			return nil, errors.New("expected ], got: " + p.current().Value)
		}

		return &Tuple{Elements: exprs}, nil
	}

	return nil, fmt.Errorf("expected identifier or [, got: %v", tok.Value)
}

func (p *Parser) parseExpression() (Expr, error) {
	left, err := p.parseTerm()
	if err != nil {
		return nil, err
	}

	for {
		var op string
		if p.eat(TokenRShift) {
			op = ">>"
		} else {
			break
		}
		right, err := p.parseTerm()
		if err != nil {
			return nil, err
		}

		left = &BinOp{Op: op, Left: left, Right: right}
	}
	return left, nil
}

func ParseDAG(input string) (Expr, error) {
	tokens, err := tokenize(input)
	if err != nil {
		return nil, err
	}

	parser := Parser{tokens, 0}
	ast, err := parser.parseExpression()
	if err != nil {
		return nil, err
	}
	if parser.current().Type != TokenEOF {
		return nil, errors.New("unexpected trailing tokens: " + parser.current().Value)
	}
	return ast, nil
}
