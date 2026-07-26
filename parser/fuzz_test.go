package parser_test

import (
	"testing"

	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/parser"
	"github.com/MoroZvlg/tascript/token"
)

var fuzzSeeds = []string{
	"",
	"function Run() {\nlet a = 1\n}",
	"function Init() {\n}\nfunction Run() {\n1\n}",
	"const K = 2\nfunction Run() {\nK + 1\n}",
	"input price: Float\nfunction Run() {\nprice\n}",
	"input btc: {foo: Integer,}\nfunction Run() {\n1\n}",
	"output sig: {dir: String, price: Float}\nfunction Run() {\nemit(sig, dir=\"up\", price=1.0)\n}",
	"state cooldown: Integer = 3\nfunction Run() {\nstate.cooldown = state.cooldown - 1\n}",
	"function Run() {\nif 1 > 0 {\n1\n}\nelse {\n2\n}\n}",
	"function Run() {\nmath.sqrt(number=9.0, )\n}",
	"function Run() {\nlet a = \"unterminated\n}",
	"function Run() {\nlet a = 1 &&\n}",
	"function Run() {\n}",
	"function nope() {\n1\n}",
	"let a = 1",
	"const A = 2\nconst A = 3\nfunction Run() {\n1\n}",
	"function Run() {\n((((((1))))))\n}",
	"function Run() {\na[0]\n}",
	"function Run() {\n(1 + 2)()\n}",
	"# comment only\n",
}

func FuzzLex(f *testing.F) {
	for _, seed := range fuzzSeeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, src string) {
		l := lexer.New(src)
		limit := len(src) + 2
		for range limit {
			if l.NextToken().Type == token.EOF {
				return
			}
		}
		t.Fatalf("no EOF after %d tokens: some token consumed zero bytes", limit)
	})
}

func FuzzParse(f *testing.F) {
	for _, seed := range fuzzSeeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, src string) {
		p := parser.New(lexer.New(src))
		prog := p.Parse()
		diags := p.Diagnostics()

		if prog.Valid != (len(diags) == 0) {
			t.Fatalf("Valid = %v with %d diagnostics: %v", prog.Valid, len(diags), diags)
		}
		if prog.Valid && prog.RunFn == nil {
			t.Fatal("valid program without a Run function")
		}
		if len(diags) > 100 {
			t.Fatalf("%d diagnostics reported, cap is 100", len(diags))
		}
	})
}
