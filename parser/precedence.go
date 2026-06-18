package parser

import (
	"github.com/MoroZvlg/tascript/token"
)

type precedence int

const (
	LowestPrec precedence = iota
	OrPrec
	AndPrec
	EqualPrec
	ComparePrec
	SumPrec
	ProductPrec
	PrefixPrec
	CallPrec
)

var precedences = map[token.TokenType]precedence{
	token.OR:       OrPrec,
	token.AND:      AndPrec,
	token.EQ:       EqualPrec,
	token.NEQ:      EqualPrec,
	token.LT:       ComparePrec,
	token.GT:       ComparePrec,
	token.LTEQ:     ComparePrec,
	token.GTEQ:     ComparePrec,
	token.PLUS:     SumPrec,
	token.MINUS:    SumPrec,
	token.ASTERISK: ProductPrec,
	token.SLASH:    ProductPrec,
	token.PERCENT:  ProductPrec,
	token.LPAREN:   CallPrec,
	token.DOT:      CallPrec,
	token.LBRACKET: CallPrec,
}
