package main

import (
	"fmt"
	"os"

	"github.com/piprim/goplt/cmd/goplt/cmd"
)

func main() {
	if err := cmd.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
