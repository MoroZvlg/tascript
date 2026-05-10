package repl_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/MoroZvlg/tascript/repl"
)

func Test_Start(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{"arithmetic", "5 + 5\n", "10"},
		{"env persists across lines", "let x = 5\nx * 2\n", "10"},
		{"const persists across lines", "const greeting = \"hi\"\ngreeting + \" you\"\n", "hi you"},
		{"runtime error doesn't crash", "foobar\n", "identifier not found: foobar"},
		{"parser error doesn't crash", "let =\n", "expected next token to be"},
		{"keeps going after error", "foobar\n1 + 1\n", "2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := bytes.NewBufferString(tt.input)
			out := bytes.NewBuffer(nil)
			repl.Start(in, out)
			if !strings.Contains(out.String(), tt.contains) {
				t.Errorf("output missing %q\nfull output:\n%s", tt.contains, out.String())
			}
		})
	}
}
