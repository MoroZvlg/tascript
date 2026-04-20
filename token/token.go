package token

import "fmt"

type TokenType string

const (
	ILLEGAL TokenType = "ILLEGAL"
	EOF     TokenType = "EOF"

	IDENT  TokenType = "IDENTIFIER"
	INT    TokenType = "INT"
	FLOAT  TokenType = "FLOAT"
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

	LPAREN    TokenType = "("
	RPAREN    TokenType = ")"
	LBRACE    TokenType = "{"
	RBRACE    TokenType = "}"
	LBRACKET  TokenType = "["
	RBRACKET  TokenType = "]"
	COMMA     TokenType = ","
	COLON     TokenType = ":"
	SEMICOLON TokenType = ";"

	LET      TokenType = "let"
	CONST    TokenType = "const"
	FUNCTION TokenType = "function"
	RETURN   TokenType = "return"
	IF       TokenType = "if"
	ELSE     TokenType = "else"
	TRUE     TokenType = "true"
	FALSE    TokenType = "false"
)

type Token struct {
	Type    TokenType
	Literal string
}

func (t *Token) String() string {
	return fmt.Sprintf("[%s] -> %s\n", t.Type, t.Literal)
}

var keywords = map[string]TokenType{
	"let":      LET,
	"const":    CONST,
	"function": FUNCTION,
	"return":   RETURN,
	"if":       IF,
	"else":     ELSE,
	"true":     TRUE,
	"false":    FALSE,
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
