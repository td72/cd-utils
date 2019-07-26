package main

import (
	"fmt"
	"path/filepath"
	"os"
	"github.com/urfave/cli"
	"github.com/b4b4r07/go-finder"
)

func exist_path(path string) bool {
	fileInfo, err := os.Stat(path)
    return !os.IsNotExist(err) && fileInfo.IsDir()
}

func list_history(c *cli.Context) error {
	info := NewInfo()
	var hs []History
	info.DB.Find(&hs)
	for _, h := range hs {
		if c.Bool("abs") {
			fmt.Println(h.Abs())
		} else {
			fmt.Println(h.Tilda())
		}
	}
	return nil
}

func update_history(_ *cli.Context) error {
	info := NewInfo()
	var hs []History
	info.DB.Find(&hs)
	for _, h := range hs {
		if !exist_path(h.Abs()) {
			info.DB.Delete(&h)
			fmt.Printf("remove: %s", h.Tilda())
		}
	}
	return nil
}

func choice_history(_ *cli.Context) error {
	info := NewInfo()
	var hs []History
	err := info.DB.Order("updated_at desc").Find(&hs).Error
	if err != nil {
		panic(err)
	}
	items := finder.NewItems()
	for _, h := range hs {
		items.Add(h.Tilda(), h)
	}
	fzf, err := finder.New("fzf", "--reverse", "--height", "40")
	if err != nil {
		panic(err)
	}
	selected, err := fzf.Select(items)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		return nil
	}
	fmt.Println(selected[0].(History).Abs())
	return nil
}

func add_history(c *cli.Context) error {
	if c.NArg() == 0 {
		return fmt.Errorf(`"history add" must need "path" argument`)
	}
	path, err := filepath.Abs(c.Args().Get(0))
	if err != nil {
		panic(err)
	}
	if !exist_path(path) {
		return fmt.Errorf("%s is not directory or does not exist", path)
	}

	info := NewInfo()
	h := NewHistory(path)
	if info.NotExist(h) {
		info.DB.Create(h)
	} else {
		info.DB.Save(h)
	}
	return nil
}

func init() {
	command := cli.Command{
		Name:    "history",
		Usage:   "cd history utils",
		Subcommands: []cli.Command{
			{
				Name: "list",
				Usage: "list all cd history",
				Action: list_history,
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name: "abs, a",
						Usage: "print absolute path. (default uses tilda)",
					},
				},
			},
			{
				Name: "update",
				Usage: "remove not existed path",
				Action: update_history,
			},
			{
				Name: "add",
				Usage: "add new path",
				Action: add_history,
			},
			{
				Name: "choice",
				Usage: "choice old path",
				Action: choice_history,
			},
		},
	}
	App.Commands = append(App.Commands, command)
}
