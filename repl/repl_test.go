package repl_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/MoroZvlg/tascript/repl"
)

func Test_Start(t *testing.T) {
	bufferIn := bytes.NewBufferString(`let x = 5;`)
	bufferOut := bytes.NewBuffer(nil)
	expectedOut := []string{
		"[let] -> let",
		"[IDENTIFIER] -> x",
		"[=] -> =",
		"[INT] -> 5",
		"[;] -> ;",
	}
	repl.Start(bufferIn, bufferOut)
	for _, w := range expectedOut {
		if !strings.Contains(bufferOut.String(), w) {
			t.Errorf("output missing %q\nfull output:\n%s", w, bufferOut.String())
		}
	}
}
