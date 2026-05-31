package main

import (
	"context"
	"os"

	"github.com/francesco/hetzner_pulumi/pkg/platform/cli"
)

func main() {
	command := cli.New(cli.CommandOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Getenv: os.Getenv,
	})

	os.Exit(command.Run(context.Background(), os.Args[1:]))
}
