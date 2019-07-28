package main

import (
	"bufio"
	"fmt"
	"github.com/b4b4r07/go-finder"
	"github.com/urfave/cli"
	"os"
	"path/filepath"
)

func list_bookmark(c *cli.Context) error {
	info := NewInfo()
	var bs []Bookmark
	info.DB.Find(&bs)
	err := info.DB.Order("path desc").Find(&bs).Error
	if err != nil {
		panic(err)
	}
	if len(bs) == 0 {
		fmt.Println("No Bookmark")
		return nil
	}
	for _, b := range bs {
		if c.Bool("abs") {
			fmt.Println(b.Abs())
		} else {
			fmt.Println(b.Tilda())
		}
	}
	return nil
}

func _choice_bookmark() (bookmark Bookmark, err error) {
	info := NewInfo()
	var bs []Bookmark
	err = info.DB.Order("path desc").Find(&bs).Error
	if err != nil {
		panic(err)
	}
	if len(bs) == 0 {
		return bookmark, fmt.Errorf("No Bookmark")
	}
	items := finder.NewItems()
	for _, b := range bs {
		items.Add(b.Tilda(), b)
	}
	fzf, err := finder.New("fzf", "--reverse", "--height", "40")
	if err != nil {
		return bookmark, err
	}
	selected, err := fzf.Select(items)
	if err != nil {
		return bookmark, err
	}
	if len(selected) == 0 {
		return bookmark, fmt.Errorf("")
	}
	return selected[0].(Bookmark), nil
}

func choice_bookmark(_ *cli.Context) error {
	p, err := _choice_bookmark()
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println(p.Abs())
	return nil
}

func _delete_bookmark(b Bookmark) error {
	info := NewInfo()
	return info.DB.Delete(&b).Error
}

func delete_bookmark(_ *cli.Context) error {
	b, err := _choice_bookmark()
	if err != nil {
		return err
	}
	fmt.Printf("%s\nregistar? [y/N]", b.Tilda())
	stdin := bufio.NewScanner(os.Stdin)
	for stdin.Scan() {
		text := stdin.Text()
		if len(text) > 0 {
			return _delete_bookmark(b)
		}
	}
	return nil
}

func add_bookmark(c *cli.Context) error {
	if c.NArg() == 0 {
		return fmt.Errorf(`"bookmark add" must need "path" argument`)
	}
	path, err := filepath.Abs(c.Args().Get(0))
	if err != nil {
		panic(err)
	}
	if !exist_path(path) {
		return fmt.Errorf("%s is not directory or does not exist", path)
	}

	info := NewInfo()
	b := NewBookmark(path)
	if info.NotExist(b) {
		info.DB.Create(b)
	} else {
		fmt.Printf("%s is already registered", b.Tilda())
	}
	return nil
}

func init() {
	command := cli.Command{
		Name:  "bookmark",
		Usage: "cd bookmark utils",
		Subcommands: []cli.Command{
			{
				Name:   "list",
				Usage:  "list all bookmarks",
				Action: list_bookmark,
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "abs, a",
						Usage: "print absolute path. (default uses tilda)",
					},
				},
			},
			{
				Name:   "add",
				Usage:  "add new bookmark",
				Action: add_bookmark,
			},
			{
				Name:   "choice",
				Usage:  "choice bookmark",
				Action: choice_bookmark,
			},
			{
				Name:   "delete",
				Usage:  "delete bookmark",
				Action: delete_bookmark,
			},
		},
	}
	App.Commands = append(App.Commands, command)
}
