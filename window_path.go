package main

import (
	"fmt"

	"github.com/urfave/cli"
)

func update_window_path(c *cli.Context) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	return store.SaveWindowPath(c.String("window_id"), c.String("path"))
}

func latest_window_path(c *cli.Context) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	wp, err := store.WindowPath(c.String("window_id"))
	if err != nil {
		return err
	}
	if wp == nil {
		return fmt.Errorf("no path for window %s", c.String("window_id"))
	}
	fmt.Println(wp.Abs())
	return nil
}

func init() {
	command := cli.Command{
		Name:  "window_path",
		Usage: "add and get window_path",
		Subcommands: []cli.Command{
			{
				Name:   "update",
				Usage:  "update path and window_id",
				Action: update_window_path,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name: "window_id, i",
					},
					cli.StringFlag{
						Name: "path, p",
					},
				},
			},
			{
				Name:   "latest",
				Usage:  "get latest path on current window",
				Action: latest_window_path,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name: "window_id, i",
					},
				},
			},
		},
	}
	App.Commands = append(App.Commands, command)
}

// vim:set fenc=utf-8 ff=unix sw=4 ts=4:
