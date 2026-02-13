//go:build !windows

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/repsejnworb/gitsej/internal/cli"
)

func main() {
	if err := cli.NewCommand().Run(context.Background(), os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
