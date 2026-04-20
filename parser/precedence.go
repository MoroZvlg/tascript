package parser

import "github.com/MoroZvlg/tascript/token"

type precedence int

const (
	LOWEST precedence = iota
	OR
	AND
	EQUALS
	COMPARE
	SUM
	PRODUCT
	PREFIX
	CALL
)

var precedences = map[token.TokenType]precedence{
	token.OR:       OR,
	token.AND:      AND,
	token.EQ:       EQUALS,
	token.NEQ:      EQUALS,
	token.LT:       COMPARE,
	token.GT:       COMPARE,
	token.LTEQ:     COMPARE,
	token.GTEQ:     COMPARE,
	token.PLUS:     SUM,
	token.MINUS:    SUM,
	token.ASTERISK: PRODUCT,
	token.SLASH:    PRODUCT,
	token.LPAREN:   CALL,
}
