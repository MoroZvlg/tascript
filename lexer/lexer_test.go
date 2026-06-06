package lexer_test

import (
	"testing"

	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/token"
)

func allTokens(t *testing.T, l *lexer.Lexer) []token.Token {
	t.Helper()
	tokens := make([]token.Token, 0)
	for {
		nextTok := l.NextToken()
		tokens = append(tokens, nextTok)
		if nextTok.Type == token.EOF {
			break
		}
	}
	return tokens
}

func TestLexer_NextToken(t *testing.T) {
	src := `// some script
const CONST_F = 2.0 // some comment
const CONST_I = 1
// more comments
//
// input eth: CandleSeries
input btc: CandleSeries
output alert: String

function Init() {
	let out = "foo"
	emit(
		alert, 
		out,
	)
}

`

	expected := []token.Token{
		{Pos: token.Pos{Line: 1, Col: 15}, Type: token.NEWLINE, Literal: ""},

		{Pos: token.Pos{Line: 2, Col: 1}, Type: token.CONST, Literal: "const"},
		{Pos: token.Pos{Line: 2, Col: 7}, Type: token.IDENT, Literal: "CONST_F"},
		{Pos: token.Pos{Line: 2, Col: 15}, Type: token.ASSIGN, Literal: "="},
		{Pos: token.Pos{Line: 2, Col: 17}, Type: token.FLOAT, Literal: "2.0"},
		{Pos: token.Pos{Line: 2, Col: 36}, Type: token.NEWLINE, Literal: ""},

		{Pos: token.Pos{Line: 3, Col: 1}, Type: token.CONST, Literal: "const"},
		{Pos: token.Pos{Line: 3, Col: 7}, Type: token.IDENT, Literal: "CONST_I"},
		{Pos: token.Pos{Line: 3, Col: 15}, Type: token.ASSIGN, Literal: "="},
		{Pos: token.Pos{Line: 3, Col: 17}, Type: token.INTEGER, Literal: "1"},
		{Pos: token.Pos{Line: 3, Col: 18}, Type: token.NEWLINE, Literal: ""},

		{Pos: token.Pos{Line: 7, Col: 1}, Type: token.INPUT, Literal: "input"},
		{Pos: token.Pos{Line: 7, Col: 7}, Type: token.IDENT, Literal: "btc"},
		{Pos: token.Pos{Line: 7, Col: 10}, Type: token.COLON, Literal: ":"},
		{Pos: token.Pos{Line: 7, Col: 12}, Type: token.IDENT, Literal: "CandleSeries"},
		{Pos: token.Pos{Line: 7, Col: 24}, Type: token.NEWLINE, Literal: ""},

		{Pos: token.Pos{Line: 8, Col: 1}, Type: token.OUTPUT, Literal: "output"},
		{Pos: token.Pos{Line: 8, Col: 8}, Type: token.IDENT, Literal: "alert"},
		{Pos: token.Pos{Line: 8, Col: 13}, Type: token.COLON, Literal: ":"},
		{Pos: token.Pos{Line: 8, Col: 15}, Type: token.IDENT, Literal: "String"},
		{Pos: token.Pos{Line: 8, Col: 21}, Type: token.NEWLINE, Literal: ""},

		{Pos: token.Pos{Line: 10, Col: 1}, Type: token.FUNCTION, Literal: "function"},
		{Pos: token.Pos{Line: 10, Col: 10}, Type: token.IDENT, Literal: "Init"},
		{Pos: token.Pos{Line: 10, Col: 14}, Type: token.LPAREN, Literal: "("},
		{Pos: token.Pos{Line: 10, Col: 15}, Type: token.RPAREN, Literal: ")"},
		{Pos: token.Pos{Line: 10, Col: 17}, Type: token.LBRACE, Literal: "{"},
		{Pos: token.Pos{Line: 10, Col: 18}, Type: token.NEWLINE, Literal: ""},

		{Pos: token.Pos{Line: 11, Col: 2}, Type: token.LET, Literal: "let"},
		{Pos: token.Pos{Line: 11, Col: 6}, Type: token.IDENT, Literal: "out"},
		{Pos: token.Pos{Line: 11, Col: 10}, Type: token.ASSIGN, Literal: "="},
		{Pos: token.Pos{Line: 11, Col: 12}, Type: token.STRING, Literal: "foo"},
		{Pos: token.Pos{Line: 11, Col: 17}, Type: token.NEWLINE, Literal: ""},

		{Pos: token.Pos{Line: 12, Col: 2}, Type: token.EMIT, Literal: "emit"},
		{Pos: token.Pos{Line: 12, Col: 6}, Type: token.LPAREN, Literal: "("},
		{Pos: token.Pos{Line: 13, Col: 3}, Type: token.IDENT, Literal: "alert"},
		{Pos: token.Pos{Line: 13, Col: 8}, Type: token.COMMA, Literal: ","},
		{Pos: token.Pos{Line: 14, Col: 3}, Type: token.IDENT, Literal: "out"},
		{Pos: token.Pos{Line: 14, Col: 6}, Type: token.COMMA, Literal: ","},
		{Pos: token.Pos{Line: 15, Col: 2}, Type: token.RPAREN, Literal: ")"},
		{Pos: token.Pos{Line: 15, Col: 3}, Type: token.NEWLINE, Literal: ""},

		{Pos: token.Pos{Line: 16, Col: 1}, Type: token.RBRACE, Literal: "}"},
		{Pos: token.Pos{Line: 16, Col: 2}, Type: token.NEWLINE, Literal: ""},

		{Pos: token.Pos{Line: 18, Col: 1}, Type: token.EOF, Literal: ""},
	}

	l := lexer.New(src)
	tokens := allTokens(t, l)
	if len(tokens) != len(expected) {
		t.Errorf("expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, tok := range tokens {
		if i >= len(expected) {
			break
		}
		expectTok := expected[i]
		if tok.Type != expectTok.Type {
			t.Errorf("expected token type '%s', got '%s'", expectTok.Type, tok.Type)
		}
		if tok.Literal != expectTok.Literal {
			t.Errorf("expected token literal '%s', got '%s'", expectTok.Literal, tok.Literal)
		}
		if tok.Pos.Line != expectTok.Pos.Line || tok.Pos.Col != expectTok.Pos.Col {
			t.Errorf("expected token position '%s', got '%s'", expectTok.Pos.String(), tok.Pos.String())
		}
	}
}

func TestLexer_Simple(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedTokens []token.TokenType
	}{
		{"ends on peek token call",
			"const foo =",
			[]token.TokenType{token.CONST, token.IDENT, token.ASSIGN, token.EOF},
		},
		{"float parsing, multiple dots",
			"1.2.3.4.5",
			[]token.TokenType{token.FLOAT, token.DOT, token.FLOAT, token.DOT, token.INTEGER, token.EOF},
		},
		{"comment line",
			"123 // comment\n foo ",
			[]token.TokenType{token.INTEGER, token.NEWLINE, token.IDENT, token.EOF},
		},
		{"empty lines",
			"123 \n\n\n foo ",
			[]token.TokenType{token.INTEGER, token.NEWLINE, token.IDENT, token.EOF},
		},
		{"newline suppressed inside brackets",
			"(1 +\n2)",
			[]token.TokenType{token.LPAREN, token.INTEGER, token.PLUS, token.INTEGER, token.RPAREN, token.EOF},
		},
		// Tripwire: a stray closer must not underflow depth and swallow newlines
		// (don't switch bracketDepth back to an unsigned type).
		{"stray closer keeps newline significant",
			")\nconst a = 1",
			[]token.TokenType{token.RPAREN, token.NEWLINE, token.CONST, token.IDENT, token.ASSIGN, token.INTEGER, token.EOF},
		},
		{"whitespace-only blank line collapses",
			"123 \n  \n foo",
			[]token.TokenType{token.INTEGER, token.NEWLINE, token.IDENT, token.EOF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			tokens := allTokens(t, l)
			if len(tokens) != len(tt.expectedTokens) {
				for _, tok := range tokens {
					t.Logf("%s", tok.String())
				}
				t.Fatalf("expected %d tokens, got %d", len(tt.expectedTokens), len(tokens))
			}

			for i, expectedTok := range tt.expectedTokens {
				gotTok := tokens[i]
				if gotTok.Type != expectedTok {
					t.Errorf("expected token %s, got %s", expectedTok, gotTok.Type)
				}
			}
		})
	}
}

func TestLexer_EmptySrc(t *testing.T) {
	src := ``
	l := lexer.New(src)
	tokens := allTokens(t, l)
	if len(tokens) != 1 {
		t.Errorf("expected 1 tokens, got %d", len(tokens))
		return
	}
	if tokens[0].Type != token.EOF {
		t.Errorf("expected token type '%s', got '%s'", token.EOF, tokens[0].Type)
	}
	// check no panic on NextToken call after EOF
	secondEOFTok := l.NextToken()
	if secondEOFTok.Type != token.EOF {
		t.Errorf("expected EOF token, got '%s'", secondEOFTok.Type)
	}
}
