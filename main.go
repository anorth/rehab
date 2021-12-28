package main

import (
	"fmt"
	"log"
	"os"

	"github.com/anorth/rehab/internal/cmd"
	"github.com/urfave/cli/v2"
)

func main() {
	// Task list:
	// - option to only propose fixes to known latest versions, avoid possibly-redundant fixes
	// - option to only change leaves in the stale graph, never update to something that needs to be updated itself (default?)
	// - option to upgrade to MVS-selected, instead of highest.
	// - command to push latest release of a specific upstream through the graph

	rehab := cmd.Rehab{
		GitHubToken:      "",
		MinimumUpgrade:   false,
		BranchPrefix:     "rehab/",
		MakePullRequests: false,
	}
	allFlag := &cli.BoolFlag{
		Name:     "all",
		Aliases:  []string{"a"},
		Usage:    "shows updates for all packages in the dependency tree",
		Required: false,
	}
	pullFlag := &cli.BoolFlag{
		Name:        "pull",
		Aliases:     nil,
		Usage:       "initiates pull requests for proposed changes (otherwise only pushes branches)",
		Required:    false,
		Destination: &rehab.MakePullRequests,
	}
	minimumFlag := &cli.BoolFlag{
		Name:        "minimum",
		Aliases:     nil,
		Usage:       "upgrades to MVS-selected version of requirements rather than the latest available",
		Required:    false,
		Destination: &rehab.MinimumUpgrade,
	}
	app := &cli.App{
		Name:     "rehab",
		HelpName: "rehab",
		Usage:    "treatment for dependencies",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "token",
				Aliases:     nil,
				Usage:       "GitHub authentication token",
				EnvVars:     []string{"GITHUB_TOKEN"},
				Required:    true,
				Destination: &rehab.GitHubToken,
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "show",
				Usage: "shows requirement updates available for a module",
				Flags: []cli.Flag{
					allFlag,
				},
				Action: func(c *cli.Context) error {
					if c.NArg() != 1 {
						return fmt.Errorf("module root required")
					}
					root := c.Args().Get(0)
					all := c.Bool("all")
					return rehab.Show(root, all)
				},
			},
			{
				Name:  "upgrade",
				Usage: "makes a pull request updating a module's requirements",
				Flags: []cli.Flag{
					allFlag,
					pullFlag,
					minimumFlag,
				},
				Action: func(c *cli.Context) error {
					if c.NArg() != 1 {
						return fmt.Errorf("module root required")
					}
					root := c.Args().Get(0)
					all := c.Bool("all")
					return rehab.Propose(c.Context, root, all)
				},
			},
		},
	}

	log.SetFlags(0)
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
