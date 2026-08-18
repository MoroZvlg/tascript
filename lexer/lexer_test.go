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

		{Pos: token.Pos{Line: 12, Col: 2}, Type: token.IDENT, Literal: "emit"},
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
			"const FOO =",
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
		{"newline suppressed before else",
			"}\nelse {",
			[]token.TokenType{token.RBRACE, token.ELSE, token.LBRACE, token.EOF},
		},
		{"newline suppressed before else across blank lines and comments",
			"} // why\n\n  else {",
			[]token.TokenType{token.RBRACE, token.ELSE, token.LBRACE, token.EOF},
		},
		{"else suppression respects identifier boundary",
			"}\nelsewhere",
			[]token.TokenType{token.RBRACE, token.NEWLINE, token.IDENT, token.EOF},
		},
		{"arithmetic operators including modulo",
			"7 % 3 * 2",
			[]token.TokenType{token.INTEGER, token.PERCENT, token.INTEGER, token.ASTERISK, token.INTEGER, token.EOF},
		},
		// Tripwire: a stray closer must not underflow depth and swallow newlines
		// (don't switch bracketDepth back to an unsigned type).
		{"stray closer keeps newline significant",
			")\nconst VALUE = 1",
			[]token.TokenType{token.RPAREN, token.NEWLINE, token.CONST, token.IDENT, token.ASSIGN, token.INTEGER, token.EOF},
		},
		{"whitespace-only blank line collapses",
			"123 \n  \n foo",
			[]token.TokenType{token.INTEGER, token.NEWLINE, token.IDENT, token.EOF},
		},
		{"NUL byte is illegal not EOF",
			"a\x00b",
			[]token.TokenType{token.IDENT, token.ILLEGAL, token.IDENT, token.EOF},
		},
		{"NUL after number keeps lexing",
			"5\x00foo",
			[]token.TokenType{token.INTEGER, token.ILLEGAL, token.IDENT, token.EOF},
		},
		// #5: a lone CR and a CRLF each act as a single NEWLINE; runs collapse like \n
		{"lone CR is a newline",
			"a\rb",
			[]token.TokenType{token.IDENT, token.NEWLINE, token.IDENT, token.EOF},
		},
		{"CRLF is a single newline",
			"a\r\nb",
			[]token.TokenType{token.IDENT, token.NEWLINE, token.IDENT, token.EOF},
		},
		{"consecutive CR collapse",
			"a\r\rb",
			[]token.TokenType{token.IDENT, token.NEWLINE, token.IDENT, token.EOF},
		},
		{"mixed LF and CR collapse",
			"a\n\rb",
			[]token.TokenType{token.IDENT, token.NEWLINE, token.IDENT, token.EOF},
		},
		// an unterminated string ends at the line break; the NEWLINE must survive or the
		// next statement merges into the bad one
		{"unterminated string stops at newline",
			"\"line1\nconst B = 5",
			[]token.TokenType{token.ILLEGAL, token.NEWLINE, token.CONST, token.IDENT, token.ASSIGN, token.INTEGER, token.EOF},
		},
		{"unterminated string stops at backslash-newline",
			"\"abc\\\nconst B = 5",
			[]token.TokenType{token.ILLEGAL, token.NEWLINE, token.CONST, token.IDENT, token.ASSIGN, token.INTEGER, token.EOF},
		},
		{"unterminated string stops at CRLF",
			"\"line1\r\nconst B = 5",
			[]token.TokenType{token.ILLEGAL, token.NEWLINE, token.CONST, token.IDENT, token.ASSIGN, token.INTEGER, token.EOF},
		},
		// a // comment must end at a lone CR too, not run to EOF
		{"comment ends at CR",
			"123 // comment\r foo",
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

func TestLexer_CarriageReturnLineTracking(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantLine int // line of the trailing `b`
	}{
		{"lone CR", "a\rb", 2},
		{"CRLF counts once", "a\r\nb", 2},
		{"two lone CR", "a\r\rb", 3},
		{"two CRLF", "a\r\n\r\nb", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := allTokens(t, lexer.New(tt.input))
			last := tokens[len(tokens)-2] // the token before EOF
			if last.Type != token.IDENT || last.Literal != "b" {
				t.Fatalf("expected trailing ident b, got %s %q", last.Type, last.Literal)
			}
			if last.Pos.Line != tt.wantLine {
				t.Errorf("trailing ident: expected line %d, got %d", tt.wantLine, last.Pos.Line)
			}
		})
	}
}

func TestLexer_StringEscapes(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantType    token.TokenType
		wantLiteral string
	}{
		{"plain", `"foo"`, token.STRING, "foo"},
		{"escaped quote", `"he said \"hi\""`, token.STRING, `he said "hi"`},
		{"escaped backslash", `"a\\b"`, token.STRING, `a\b`},
		{"newline escape", `"a\nb"`, token.STRING, "a\nb"},
		{"tab escape", `"a\tb"`, token.STRING, "a\tb"},
		{"carriage return escape", `"a\rb"`, token.STRING, "a\rb"},
		{"invalid escape recovers to closing quote", `"a\qb"`, token.ILLEGAL, "invalid escape sequence"},
		{"embedded NUL is illegal", "\"a\x00b\"", token.ILLEGAL, "illegal NUL byte"},
		{"trailing backslash", `"abc\`, token.ILLEGAL, "unterminated string"},
		{"unterminated at EOF", `"abc`, token.ILLEGAL, "unterminated string"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// One literal must lex to exactly one token (+ EOF): a bad escape or NUL
			// recovers to the closing quote instead of cascading into trailing tokens.
			tokens := allTokens(t, lexer.New(tt.input))
			if len(tokens) != 2 {
				t.Fatalf("expected 1 token + EOF, got %d: %v", len(tokens), tokens)
			}
			if tokens[0].Type != tt.wantType {
				t.Fatalf("expected %s, got %s (%q)", tt.wantType, tokens[0].Type, tokens[0].Literal)
			}
			if tokens[0].Literal != tt.wantLiteral {
				t.Errorf("literal: expected %q, got %q", tt.wantLiteral, tokens[0].Literal)
			}
		})
	}
}
