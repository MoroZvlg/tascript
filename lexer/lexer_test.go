package lexer_test

import (
	"testing"

	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/token"
)

func TestLexer_NextToken(t *testing.T) {
	input := `let five= 5 ;
  const ten = 10;
  function add(x, y) { x + y; }
  sma(close, 14)`
	expectedOut := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.LET, "let"},
		{token.IDENT, "five"},
		{token.ASSIGN, "="},
		{token.INT, "5"},
		{token.SEMICOLON, ";"},

		{token.CONST, "const"},
		{token.IDENT, "ten"},
		{token.ASSIGN, "="},
		{token.INT, "10"},
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

		{token.IDENT, "sma"},
		{token.LPAREN, "("},
		{token.IDENT, "close"},
		{token.COMMA, ","},
		{token.INT, "14"},
		{token.RPAREN, ")"},

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
