package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/repsejnworb/gitsej/internal/gitsej"
	cli "github.com/urfave/cli/v3"
)

type envDefaults struct {
	MainWorktree bool   `env:"GITSEJ_MAIN_WORKTREE" envDefault:"false"`
	MainBranch   string `env:"GITSEJ_MAIN_BRANCH" envDefault:"main"`
}

func NewCommand() *cli.Command {
	defaults := envDefaults{}
	if err := env.Parse(&defaults); err != nil {
		defaults = envDefaults{
			MainWorktree: false,
			MainBranch:   "main",
		}
	}

	return &cli.Command{
		Name:      "gitsej",
		Usage:     "create a gitsej repo directory from a remote Git URL",
		UsageText: "gitsej [options] <repo-url> [directory]",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "main-worktree",
				Usage: "create a main worktree checkout at <directory>/main",
				Value: defaults.MainWorktree,
			},
			&cli.StringFlag{
				Name:  "main-branch",
				Usage: "branch name for main worktree creation and .gitsej defaults",
				Value: defaults.MainBranch,
			},
		},
		Action: run,
	}
}

func run(ctx context.Context, c *cli.Command) error {
	args := c.Args().Slice()
	if len(args) < 1 || len(args) > 2 {
		return cli.Exit("expected <repo-url> [directory]", 2)
	}

	targetDir := ""
	if len(args) == 2 {
		targetDir = strings.TrimSpace(args[1])
	}

	createdDir, err := gitsej.Create(ctx, gitsej.CreateOptions{
		RepoURL:      strings.TrimSpace(args[0]),
		Directory:    targetDir,
		MainWorktree: c.Bool("main-worktree"),
		MainBranch:   strings.TrimSpace(c.String("main-branch")),
	})
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(outputWriter(c), "created gitsej repo: %s\n", createdDir)
	return err
}

func outputWriter(c *cli.Command) io.Writer {
	root := c.Root()
	if root != nil && root.Writer != nil {
		return root.Writer
	}
	if c.Writer != nil {
		return c.Writer
	}
	return os.Stdout
}
