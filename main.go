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
	// - option to only suggest release versions, not git checkpoints
	// - option to only propose fixes to known latest versions, avoid possibly-redundant fixes
	// - option to only change leaves in the stale graph, never update to something that needs to be updated itself (default?)
	// - command to push latest release of a specific upstream through the graph
	// - command to push version upgrades to a single module

	rehab := cmd.Rehab{}
	app := &cli.App{
		Name:     "rehab",
		HelpName: "rehab",
		Usage:    "treatment for dependencies",
		Commands: []*cli.Command{
			{
				Name:  "show",
				Usage: "shows upgrades to a module",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:     "all",
						Aliases:  []string{"a"},
						Usage:    "show updates for all packages in the dependency tree",
						Required: false,
					},
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
				Name:  "propose",
				Usage: "makes pull requests initiating upgrades to a module",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:     "all",
						Aliases:  []string{"a"},
						Usage:    "propose upgrades for all packages in the dependency tree",
						Required: false,
					},
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
