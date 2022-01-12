package main

import (
	"fmt"

	"github.com/urfave/cli"
)

func update_window_path(c *cli.Context) error {
	window_id := c.String("window_id")
	path := c.String("path")

	info := NewInfo()
	var wp WindowPath
	err := info.DB.Where(
		"window_id = ?",
		window_id,
	).First(&wp).Error

	if err != nil {
		p := NewWindowPath(window_id, path)
		info.DB.Create(p)
	} else {
		new_path, is_home := convert_to_tilda_path(path)
		wp.Path.Path = new_path
		wp.Path.IsHome = is_home
		// wp.Path = &Path{Path: new_path, IsHome: is_home}
		info.DB.Save(wp)
	}
	return nil
}

func latest_window_path(c *cli.Context) error {
	info := NewInfo()
	var wp WindowPath
	window_id := c.String("window_id")
	err := info.DB.Where(
		"window_id = ?",
		window_id,
	).First(&wp).Error

	if err != nil {
		return err
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
