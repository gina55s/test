package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/threefoldtech/test/cmds/modules/capacityd"
	"github.com/threefoldtech/test/cmds/modules/contd"
	"github.com/threefoldtech/test/cmds/modules/flistd"
	"github.com/threefoldtech/test/cmds/modules/gateway"
	"github.com/threefoldtech/test/cmds/modules/networkd"
	"github.com/threefoldtech/test/cmds/modules/provisiond"
	"github.com/threefoldtech/test/cmds/modules/qsfsd"
	"github.com/threefoldtech/test/cmds/modules/storaged"
	"github.com/threefoldtech/test/cmds/modules/vmd"
	"github.com/threefoldtech/test/cmds/modules/zbusdebug"
	"github.com/threefoldtech/test/cmds/modules/zui"
	"github.com/threefoldtech/test/pkg/app"
	"github.com/threefoldtech/test/pkg/version"
	"github.com/urfave/cli/v2"
)

func main() {
	app.Initialize()

	exe := cli.App{
		Name:    "ZOS",
		Version: version.Current().String(),
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:   "list",
				Hidden: true,
				Usage:  "print all available clients names",
			},
		},
		Commands: []*cli.Command{
			&zui.Module,
			&storaged.Module,
			&flistd.Module,
			&contd.Module,
			&vmd.Module,
			&capacityd.Module,
			&networkd.Module,
			&provisiond.Module,
			&zbusdebug.Module,
			&gateway.Module,
			&qsfsd.Module,
		},
		Action: func(c *cli.Context) error {
			if !c.Bool("list") {
				cli.ShowAppHelpAndExit(c, 0)
			}
			// this hidden flag (complete) is used to list
			// all available modules names to automate building of
			// symlinks
			for _, cmd := range c.App.VisibleCommands() {
				if cmd.Name == "help" {
					continue
				}
				fmt.Println(cmd.Name)
			}
			return nil
		},
	}

	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Println(c.App.Version)
	}
	name := filepath.Base(os.Args[0])
	args := os.Args
	for _, cmd := range exe.Commands {
		if cmd.Name == name {
			args = make([]string, 0, len(os.Args)+1)
			// this converts /bin/name <args> to  'test <name> <args>
			args = append(args, "bin", name)
			args = append(args, os.Args[1:]...)
			break
		}
	}

	if err := exe.Run(args); err != nil {
		log.Fatal().Err(err).Msg("exiting")
	}
}
