package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/b4b4r07/go-finder"
	"github.com/urfave/cli"
)

// isDir は path が存在するディレクトリかどうかを返す。
//
// 従来の exist_path は os.Stat が NotExist 以外のエラー (権限不足など) を
// 返したときに nil の FileInfo を参照して panic していた。
func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func list_history(c *cli.Context) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	hs, err := store.Histories()
	if err != nil {
		return err
	}
	for _, h := range hs {
		if c.Bool("abs") {
			fmt.Println(h.Abs())
		} else {
			fmt.Println(h.Tilde())
		}
	}
	return nil
}

func update_history(_ *cli.Context) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	hs, err := store.Histories()
	if err != nil {
		return err
	}
	for i := range hs {
		if isDir(hs[i].Abs()) {
			continue
		}
		if err := store.DeleteHistory(&hs[i]); err != nil {
			return err
		}
		fmt.Printf("remove: %s", hs[i].Tilde())
	}
	return nil
}

func choice_history(_ *cli.Context) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	hs, err := store.Histories()
	if err != nil {
		return err
	}
	items := finder.NewItems()
	for _, h := range hs[1:] {
		items.Add(h.Tilde(), h)
	}
	fzf, err := finder.New("fzf", "--reverse", "--height", "40")
	if err != nil {
		return err
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

	return store.AddHistory(path)
}

func latest_history(_ *cli.Context) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	hs, err := store.Histories()
	if err != nil {
		return err
	}
	if len(hs) > 1 {
		fmt.Println(hs[1].Abs())
	}
	return nil
}

func init() {
	command := cli.Command{
		Name:  "history",
		Usage: "cd history utils",
		Subcommands: []cli.Command{
			{
				Name:   "list",
				Usage:  "list all cd history",
				Action: list_history,
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "abs, a",
						Usage: "print absolute path. (default uses tilda)",
					},
				},
			},
			{
				Name:   "update",
				Usage:  "remove not existed path",
				Action: update_history,
			},
			{
				Name:   "add",
				Usage:  "add new path",
				Action: add_history,
			},
			{
				Name:   "choice",
				Usage:  "choice old path",
				Action: choice_history,
			},
			{
				Name:   "latest",
				Usage:  "latest path",
				Action: latest_history,
			},
		},
	}
	App.Commands = append(App.Commands, command)
}
