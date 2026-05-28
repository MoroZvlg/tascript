package token

import "fmt"

type TokenType string

const (
	ILLEGAL TokenType = "ILLEGAL"
	EOF     TokenType = "EOF"
	NEWLINE TokenType = "NEWLINE"

	IDENT  TokenType = "IDENTIFIER"
	NUMBER TokenType = "NUMBER"
	STRING TokenType = "STRING"

	ASSIGN   TokenType = "="
	EQ       TokenType = "=="
	NEQ      TokenType = "!="
	LT       TokenType = "<"
	GT       TokenType = ">"
	LTEQ     TokenType = "<="
	GTEQ     TokenType = ">="
	PLUS     TokenType = "+"
	MINUS    TokenType = "-"
	BANG     TokenType = "!"
	ASTERISK TokenType = "*"
	SLASH    TokenType = "/"
	AND      TokenType = "&&"
	OR       TokenType = "||"

	LPAREN   TokenType = "("
	RPAREN   TokenType = ")"
	LBRACE   TokenType = "{"
	RBRACE   TokenType = "}"
	LBRACKET TokenType = "["
	RBRACKET TokenType = "]"
	COMMA    TokenType = ","
	COLON    TokenType = ":"
	DOT      TokenType = "."

	LET      TokenType = "let"
	CONST    TokenType = "const"
	INPUT    TokenType = "input"
	OUTPUT   TokenType = "output"
	EMIT     TokenType = "emit"
	FUNCTION TokenType = "function"
	IF       TokenType = "if"
	ELSE     TokenType = "else"
	TRUE     TokenType = "true"
	FALSE    TokenType = "false"
)

type Pos struct {
	Line int
	Col  int
}

func (p *Pos) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Col)
}

type Token struct {
	Pos     Pos
	Type    TokenType
	Literal string
}

func (t *Token) String() string {
	return fmt.Sprintf("[%s] -> %s\n", t.Type, t.Literal)
}

var keywordsMap = map[string]TokenType{
	"let":      LET,
	"const":    CONST,
	"input":    INPUT,
	"output":   OUTPUT,
	"emit":     EMIT,
	"function": FUNCTION,
	"if":       IF,
	"else":     ELSE,
	"true":     TRUE,
	"false":    FALSE,
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywordsMap[ident]; ok {
		return tok
	}
	return IDENT
}
