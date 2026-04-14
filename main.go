package main

import (
	"fmt"
	"os"
	"os/user"

	"github.com/MoroZvlg/tascript/repl"
)

func main() {
	u, err := user.Current()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Hello %s! Welcome to tascript.\n", u.Username)
	fmt.Println("Type away:")
	repl.Start(os.Stdin, os.Stdout)
}
