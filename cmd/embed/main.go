package main

import (
	"fmt"
	"os"

	"github.com/karutselvan/chat-assistant/cmd/embed/embedcmd"
)

func main() {

	if err := embedcmd.RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

}
