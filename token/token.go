package token

import (
	"fmt"
	"slices"
)

//nolint:revive // TokenType is the established spelling in this codebase
type TokenType string

const (
	ILLEGAL TokenType = "ILLEGAL"
	EOF     TokenType = "EOF"
	NEWLINE TokenType = "NEWLINE"

	IDENT   TokenType = "IDENTIFIER"
	INTEGER TokenType = "INTEGER"
	FLOAT   TokenType = "FLOAT"
	STRING  TokenType = "STRING"

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
	PERCENT  TokenType = "%"
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

func (p Pos) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Col)
}

type Token struct {
	Pos     Pos
	Type    TokenType
	Literal string
}

func (t Token) String() string {
	return fmt.Sprintf("[%s] -> %s", t.Type, t.Literal)
}

var keywordsMap = map[string]TokenType{
	"let":      LET,
	"const":    CONST,
	"input":    INPUT,
	"output":   OUTPUT,
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

func IsKeyword(ident string) bool {
	_, ok := keywordsMap[ident]
	return ok
}

// not in keywordsMap: the lexer emits these as IDENT
var reservedIdents = []string{"emit", "Init", "Run"}

func ReservedIdents() []string {
	return slices.Clone(reservedIdents)
}

func IsReservedIdent(ident string) bool {
	return slices.Contains(reservedIdents, ident)
}

func IsIdentStart(ch byte) bool {
	return ('a' <= ch && ch <= 'z') || ('A' <= ch && ch <= 'Z') || (ch == '_')
}

func IsIdentPart(ch byte) bool {
	return IsIdentStart(ch) || ('0' <= ch && ch <= '9')
}

func IsIdent(s string) bool {
	if s == "" || !IsIdentStart(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !IsIdentPart(s[i]) {
			return false
		}
	}
	return true
}
