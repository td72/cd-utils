package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli"
)

func list_bookmark(c *cli.Context) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	bs, err := store.Bookmarks()
	if err != nil {
		return err
	}
	if len(bs) == 0 {
		fmt.Println("No Bookmark")
		return nil
	}
	for _, b := range bs {
		if c.Bool("abs") {
			fmt.Println(b.Abs())
		} else {
			fmt.Println(b.Tilde())
		}
	}
	return nil
}

func _choice_bookmark(store *Store) (bookmark Bookmark, err error) {
	bs, err := store.Bookmarks()
	if err != nil {
		return bookmark, err
	}
	if len(bs) == 0 {
		return bookmark, fmt.Errorf("No Bookmark")
	}
	labels := make([]string, len(bs))
	for i, b := range bs {
		labels[i] = b.Tilde()
	}

	idx, err := selectWithFzf(labels)
	if err != nil {
		return bookmark, err
	}
	if idx < 0 {
		return bookmark, fmt.Errorf("")
	}
	return bs[idx], nil
}

func choice_bookmark(_ *cli.Context) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	p, err := _choice_bookmark(store)
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println(p.Abs())
	return nil
}

func delete_bookmark(_ *cli.Context) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	b, err := _choice_bookmark(store)
	if err != nil {
		return err
	}
	fmt.Printf("%s\nregistar? [y/N]", b.Tilde())
	stdin := bufio.NewScanner(os.Stdin)
	for stdin.Scan() {
		text := stdin.Text()
		if len(text) > 0 {
			return store.DeleteBookmark(&b)
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
		return err
	}
	if !isDir(path) {
		return fmt.Errorf("%s is not directory or does not exist", path)
	}

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	created, err := store.AddBookmark(path)
	if err != nil {
		return err
	}
	if !created {
		p, isHome := toTildePath(path)
		fmt.Printf("%s is already registered", Path{Path: p, IsHome: isHome}.Tilde())
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
