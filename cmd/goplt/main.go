package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/piprim/goplt/cmd/goplt/cmd"
)

func main() {
	err := cmd.NewRootCmd().Execute()
	if err == nil {
		os.Exit(0)
	}

	if exitErr, ok := errors.AsType[*cmd.ExitCodeError](err); ok {
		os.Exit(exitErr.Code)
	}

	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
