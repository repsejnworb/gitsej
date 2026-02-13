package cli

import (
	"bufio"
	"context"
	"errors"
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
		Usage:     "bootstrap and initialize gitsej repos",
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
		Commands: []*cli.Command{
			{
				Name:      "init",
				Usage:     "initialize .git/.gitsej in an existing gitsej repo directory",
				UsageText: "gitsej init [options] [directory]",
				Action:    runInit,
			},
			{
				Name:      "migrate",
				Usage:     "migrate a standard clone into a gitsej repo directory",
				UsageText: "gitsej migrate [options] <directory>",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "yes",
						Aliases: []string{"y"},
						Usage:   "proceed even if main worktree has uncommitted changes",
					},
				},
				Action: runMigrate,
			},
		},
		Action: runCreate,
	}
}

func runCreate(ctx context.Context, c *cli.Command) error {
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

func runInit(_ context.Context, c *cli.Command) error {
	args := c.Args().Slice()
	if len(args) > 1 {
		return cli.Exit("expected [directory]", 2)
	}

	targetDir := "."
	if len(args) == 1 {
		targetDir = strings.TrimSpace(args[0])
	}

	result, err := gitsej.Init(gitsej.InitOptions{
		Directory:  targetDir,
		MainBranch: strings.TrimSpace(c.String("main-branch")),
	})
	if err != nil {
		return err
	}

	created := make([]string, 0, 2)
	if result.CreatedGitFile {
		created = append(created, ".git")
	}
	if result.CreatedConfig {
		created = append(created, ".gitsej")
	}

	if len(created) == 0 {
		_, err = fmt.Fprintf(outputWriter(c), "initialized gitsej repo: %s (no changes)\n", result.Directory)
		return err
	}

	_, err = fmt.Fprintf(
		outputWriter(c),
		"initialized gitsej repo: %s (created %s)\n",
		result.Directory,
		strings.Join(created, ", "),
	)
	return err
}

func runMigrate(ctx context.Context, c *cli.Command) error {
	args := c.Args().Slice()
	if len(args) != 1 {
		return cli.Exit("expected <directory>", 2)
	}

	opts := gitsej.MigrateOptions{
		Directory:      strings.TrimSpace(args[0]),
		ForceMainClean: c.Bool("yes"),
	}
	if c.IsSet("main-branch") {
		opts.MainBranch = strings.TrimSpace(c.String("main-branch"))
	}

	result, err := gitsej.Migrate(ctx, opts)
	if err != nil {
		var dirtyErr *gitsej.DirtyMainWorktreeError
		if errors.As(err, &dirtyErr) && !opts.ForceMainClean {
			confirmed, confirmErr := confirmMainCleanup(c, dirtyErr.Path)
			if confirmErr != nil {
				return confirmErr
			}
			if !confirmed {
				return errors.New("migration canceled")
			}
			opts.ForceMainClean = true
			result, err = gitsej.Migrate(ctx, opts)
		}
		if err != nil {
			return err
		}
	}

	createdConfig := "no"
	if result.CreatedConfig {
		createdConfig = "yes"
	}

	if _, err := fmt.Fprintf(
		outputWriter(c),
		"migrated gitsej repo: %s (main_branch=%s, created_.gitsej=%s, moved_worktrees=%d)\n",
		result.Directory,
		result.MainBranch,
		createdConfig,
		len(result.MovedWorktrees),
	); err != nil {
		return err
	}
	for _, moved := range result.MovedWorktrees {
		if _, err := fmt.Fprintf(outputWriter(c), "moved worktree: %s\n", moved); err != nil {
			return err
		}
	}
	return nil
}

func confirmMainCleanup(c *cli.Command, path string) (bool, error) {
	if _, err := fmt.Fprintf(
		outputWriter(c),
		"main worktree is dirty and will be cleaned during migration: %s\ncontinue? [y/N]: ",
		path,
	); err != nil {
		return false, err
	}

	reader := bufio.NewReader(inputReader(c))
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
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

func inputReader(c *cli.Command) io.Reader {
	root := c.Root()
	if root != nil && root.Reader != nil {
		return root.Reader
	}
	if c.Reader != nil {
		return c.Reader
	}
	return os.Stdin
}
