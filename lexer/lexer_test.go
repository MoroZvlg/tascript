package lexer_test

import (
	"testing"

	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/token"
)

func TestLexer_NextToken(t *testing.T) {
	input := `let five= 5 ;
// some comment
let empty = "";
let filled_str = "filled";
const ten = 10.0;
function add(x, y) { x + y; }
if (sma(close, 14) > 50.0) {
	five = 10
} else {
	five = 5.0
}
foo.bar`
	expectedOut := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.LET, "let"},
		{token.IDENT, "five"},
		{token.ASSIGN, "="},
		{token.INT, "5"},
		{token.SEMICOLON, ";"},

		{token.LET, "let"},
		{token.IDENT, "empty"},
		{token.ASSIGN, "="},
		{token.STRING, ""},
		{token.SEMICOLON, ";"},

		{token.LET, "let"},
		{token.IDENT, "filled_str"},
		{token.ASSIGN, "="},
		{token.STRING, "filled"},
		{token.SEMICOLON, ";"},

		{token.CONST, "const"},
		{token.IDENT, "ten"},
		{token.ASSIGN, "="},
		{token.FLOAT, "10.0"},
		{token.SEMICOLON, ";"},

		{token.FUNCTION, "function"},
		{token.IDENT, "add"},
		{token.LPAREN, "("},
		{token.IDENT, "x"},
		{token.COMMA, ","},
		{token.IDENT, "y"},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.IDENT, "x"},
		{token.PLUS, "+"},
		{token.IDENT, "y"},
		{token.SEMICOLON, ";"},
		{token.RBRACE, "}"},

		{token.IF, "if"},
		{token.LPAREN, "("},
		{token.IDENT, "sma"},
		{token.LPAREN, "("},
		{token.IDENT, "close"},
		{token.COMMA, ","},
		{token.INT, "14"},
		{token.RPAREN, ")"},
		{token.GT, ">"},
		{token.FLOAT, "50.0"},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.IDENT, "five"},
		{token.ASSIGN, "="},
		{token.INT, "10"},
		{token.RBRACE, "}"},
		{token.ELSE, "else"},
		{token.LBRACE, "{"},
		{token.IDENT, "five"},
		{token.ASSIGN, "="},
		{token.FLOAT, "5.0"},
		{token.RBRACE, "}"},

		{token.IDENT, "foo"},
		{token.DOT, "."},
		{token.IDENT, "bar"},

		{token.EOF, ""},
	}
	lex := lexer.New(input)
	for i, tt := range expectedOut {
		tok := lex.NextToken()
		if tok.Type != tt.expectedType {
			t.Errorf("(%d) expected token type %v -> got %v", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Errorf("(%d) expected literal %q -> got %q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestLexer_IdentifierWithDigits(t *testing.T) {
	input := `s14 sma7_now r2d2 14foo`
	expected := []struct {
		typ token.TokenType
		lit string
	}{
		{token.IDENT, "s14"},
		{token.IDENT, "sma7_now"},
		{token.IDENT, "r2d2"},
		{token.INT, "14"},
		{token.IDENT, "foo"},
		{token.EOF, ""},
	}
	lex := lexer.New(input)
	for i, want := range expected {
		tok := lex.NextToken()
		if tok.Type != want.typ {
			t.Errorf("(%d) expected type %v, got %v (literal %q)", i, want.typ, tok.Type, tok.Literal)
		}
		if tok.Literal != want.lit {
			t.Errorf("(%d) expected literal %q, got %q", i, want.lit, tok.Literal)
		}
	}
}
