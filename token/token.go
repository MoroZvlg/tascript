package token

import "fmt"

type Kind int

const (
	ILLEGAL Kind = iota
	EOF
	NEWLINE

	IDENT
	NUMBER
	STRING

	ASSIGN
	COLON
	COMMA
	DOT

	PLUS
	MINUS
	ASTERISK
	SLASH
	PERCENT
	BANG
	EQ
	NEQ
	LT
	LTE
	GT
	GTE
	AND
	OR

	LPAREN
	RPAREN
	LBRACE
	RBRACE
	LBRACKET
	RBRACKET
)

func (k Kind) String() string {
	switch k {
	case ILLEGAL:
		return "ILLEGAL"
	case EOF:
		return "EOF"
	case NEWLINE:
		return "NEWLINE"
	case IDENT:
		return "IDENT"
	case NUMBER:
		return "NUMBER"
	case STRING:
		return "STRING"
	case ASSIGN:
		return "="
	case COLON:
		return ":"
	case COMMA:
		return ","
	case DOT:
		return "."
	case PLUS:
		return "+"
	case MINUS:
		return "-"
	case ASTERISK:
		return "*"
	case SLASH:
		return "/"
	case PERCENT:
		return "%"
	case BANG:
		return "!"
	case EQ:
		return "=="
	case NEQ:
		return "!="
	case LT:
		return "<"
	case LTE:
		return "<="
	case GT:
		return ">"
	case GTE:
		return ">="
	case AND:
		return "&&"
	case OR:
		return "||"
	case LPAREN:
		return "("
	case RPAREN:
		return ")"
	case LBRACE:
		return "{"
	case RBRACE:
		return "}"
	case LBRACKET:
		return "["
	case RBRACKET:
		return "]"
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

type Pos struct {
	File   string
	Line   int
	Column int
}

func (p Pos) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

type Token struct {
	Kind    Kind
	Literal string
	Pos     Pos
}

func (t Token) String() string {
	if t.Literal == "" {
		return t.Kind.String()
	}
	return fmt.Sprintf("%s(%q)", t.Kind, t.Literal)
}
