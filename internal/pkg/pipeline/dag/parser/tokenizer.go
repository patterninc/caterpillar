package parser

import "fmt"

type TokenType int

const (
	TokenEOF TokenType = iota
	TokenIdent
	TokenRShift   // >>
	TokenLBracket // [
	TokenRBracket // ]
	TokenComma    // ,
)

type Token struct {
	Type  TokenType
	Value string
}

func tokenize(input string) ([]Token, error) {
	var tokens []Token
	pos := 0
	for pos < len(input) {
		switch {
		case input[pos] == ' ' || input[pos] == '\t' || input[pos] == '\n':
			pos++
		case input[pos] == '[':
			tokens = append(tokens, Token{TokenLBracket, "["})
			pos++
		case input[pos] == ']':
			tokens = append(tokens, Token{TokenRBracket, "]"})
			pos++
		case pos+2 <= len(input) && input[pos:pos+2] == ">>":
			tokens = append(tokens, Token{TokenRShift, ">>"})
			pos += 2
		case (input[pos] >= 'a' && input[pos] <= 'z') || (input[pos] >= 'A' && input[pos] <= 'Z') || input[pos] == '_':
			start := pos
			pos++
			for pos < len(input) && ((input[pos] >= 'a' && input[pos] <= 'z') ||
				(input[pos] >= 'A' && input[pos] <= 'Z') ||
				(input[pos] >= '0' && input[pos] <= '9') ||
				input[pos] == '_') {
				pos++
			}
			tokens = append(tokens, Token{TokenIdent, input[start:pos]})
		case input[pos] == ',':
			tokens = append(tokens, Token{TokenComma, ","})
			pos++
		default:
			return nil, fmt.Errorf("unexpected character '%c'", input[pos])
		}
	}
	tokens = append(tokens, Token{TokenEOF, ""})
	return tokens, nil
}
