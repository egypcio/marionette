package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

var ErrUsage = errors.New("usage")

func main() {
	if err := run(os.Args[1:]); err == ErrUsage {
		fmt.Fprintln(os.Stderr, Usage())
		os.Exit(1)
	} else if err == flag.ErrHelp {
		os.Exit(1)
	} else if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return ErrUsage
	}

	switch args[0] {
	case "client":
		return NewClientCommand().Run(args[1:])
	case "server":
		return NewServerCommand().Run(args[1:])
	default:
		return ErrUsage
	}
}

func Usage() string {
	return `
Marionette is a programmable client-server proxy that enables the user to
control network traffic features with a lightweight domain-specific language.

Usage:

	marionette command [arguments]

The commands are:

	client  runs the client proxy
	server  runs the server proxy
`[1:]
}