package lexer_test

import (
	"testing"

	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/token"
)

func TestLexer_Greeting(t *testing.T) {
	src := []byte(`GREETING = "hello world"

output alerts {
  message: String
}

function Init() {}
function Run() {
  emit(alerts, message=GREETING)
}
`)
	got := lexer.New(src, "greeting.tas").Tokenize()

	wantKinds := []token.Kind{
		token.IDENT, token.ASSIGN, token.STRING, token.NEWLINE,
		token.IDENT, token.IDENT, token.LBRACE, token.NEWLINE,
		token.IDENT, token.COLON, token.IDENT, token.NEWLINE,
		token.RBRACE, token.NEWLINE,
		token.IDENT, token.IDENT, token.LPAREN, token.RPAREN, token.LBRACE, token.RBRACE, token.NEWLINE,
		token.IDENT, token.IDENT, token.LPAREN, token.RPAREN, token.LBRACE, token.NEWLINE,
		token.IDENT, token.LPAREN, token.IDENT, token.COMMA, token.IDENT, token.ASSIGN, token.IDENT, token.RPAREN, token.NEWLINE,
		token.RBRACE, token.NEWLINE,
		token.EOF,
	}
	if len(got) != len(wantKinds) {
		t.Fatalf("token count: got %d, want %d\ntokens: %#v", len(got), len(wantKinds), got)
	}
	for i, k := range wantKinds {
		if got[i].Kind != k {
			t.Errorf("tok[%d].Kind = %s, want %s (lit=%q)", i, got[i].Kind, k, got[i].Literal)
		}
	}
}

func TestLexer_NumberAndComment(t *testing.T) {
	src := []byte(`X = 14
Y = 1.5  // half
`)
	got := lexer.New(src, "").Tokenize()
	literals := []string{}
	for _, t := range got {
		if t.Kind == token.NUMBER {
			literals = append(literals, t.Literal)
		}
	}
	want := []string{"14", "1.5"}
	if len(literals) != len(want) {
		t.Fatalf("numbers: got %v, want %v", literals, want)
	}
	for i, lit := range want {
		if literals[i] != lit {
			t.Errorf("num[%d] = %q, want %q", i, literals[i], lit)
		}
	}
}

func TestLexer_OperatorsAndPostfixTokens(t *testing.T) {
	src := []byte(`a == b != c <= d >= e && !x || y[1].z + 2*3 / 4 % 5
`)
	got := lexer.New(src, "").Tokenize()
	wantKinds := []token.Kind{
		token.IDENT, token.EQ, token.IDENT, token.NEQ, token.IDENT,
		token.LTE, token.IDENT, token.GTE, token.IDENT, token.AND,
		token.BANG, token.IDENT, token.OR, token.IDENT, token.LBRACKET,
		token.NUMBER, token.RBRACKET, token.DOT, token.IDENT, token.PLUS,
		token.NUMBER, token.ASTERISK, token.NUMBER, token.SLASH,
		token.NUMBER, token.PERCENT, token.NUMBER, token.NEWLINE, token.EOF,
	}
	if len(got) != len(wantKinds) {
		t.Fatalf("token count: got %d, want %d\ntokens: %#v", len(got), len(wantKinds), got)
	}
	for i, k := range wantKinds {
		if got[i].Kind != k {
			t.Errorf("tok[%d].Kind = %s, want %s (lit=%q)", i, got[i].Kind, k, got[i].Literal)
		}
	}
}

func TestLexer_UnterminatedString(t *testing.T) {
	src := []byte(`X = "oops
`)
	l := lexer.New(src, "")
	l.Tokenize()
	if len(l.Errors()) == 0 {
		t.Fatalf("expected lexer error for unterminated string")
	}
}

func TestLexer_PosTracking(t *testing.T) {
	src := []byte("X = 1\nY = 2\n")
	got := lexer.New(src, "f.tas").Tokenize()
	if got[0].Pos.Line != 1 || got[0].Pos.Column != 1 {
		t.Errorf("X pos = %s, want 1:1", got[0].Pos)
	}
	// after NEWLINE comes Y on line 2
	var y token.Token
	for i, tk := range got {
		if tk.Kind == token.NEWLINE && i+1 < len(got) {
			y = got[i+1]
			break
		}
	}
	if y.Pos.Line != 2 || y.Pos.Column != 1 {
		t.Errorf("Y pos = %s, want 2:1", y.Pos)
	}
}
