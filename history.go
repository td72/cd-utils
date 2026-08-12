package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"
)

// isDir は path が存在するディレクトリかどうかを返す。
//
// 従来実装は os.Stat が NotExist 以外のエラー (権限不足など) を返したときに
// nil の FileInfo を参照して panic していた。
func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func historyCommand() *cli.Command {
	return &cli.Command{
		Name:  "history",
		Usage: "cd history utils",
		Commands: []*cli.Command{
			{
				Name:   "list",
				Usage:  "list all cd history",
				Action: listHistory,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "abs",
						Aliases: []string{"a"},
						Usage:   "print absolute path. (default uses tilde)",
					},
				},
			},
			{
				Name:   "update",
				Usage:  "remove not existed path",
				Action: updateHistory,
			},
			{
				Name:      "add",
				Usage:     "add new path",
				ArgsUsage: "<path>",
				Action:    addHistory,
			},
			{
				Name:   "choice",
				Usage:  "choice old path",
				Action: choiceHistory,
			},
			{
				Name:   "latest",
				Usage:  "print the previous directory",
				Action: latestHistory,
			},
		},
	}
}

func listHistory(_ context.Context, cmd *cli.Command) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	hs, err := store.Histories()
	if err != nil {
		return err
	}
	abs := cmd.Bool("abs")
	for _, h := range hs {
		if abs {
			fmt.Println(h.Abs())
		} else {
			fmt.Println(h.Tilde())
		}
	}
	return nil
}

func updateHistory(_ context.Context, _ *cli.Command) error {
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
		// stdout はコマンド置換で読まれるので、通知は stderr に出す。
		fmt.Fprintf(os.Stderr, "remove: %s\n", hs[i].Tilde())
	}
	return nil
}

func addHistory(_ context.Context, cmd *cli.Command) error {
	if cmd.NArg() == 0 {
		return errors.New(`"history add" requires a <path> argument`)
	}
	path, err := filepath.Abs(cmd.Args().Get(0))
	if err != nil {
		return err
	}
	if !isDir(path) {
		return fmt.Errorf("%s is not a directory or does not exist", path)
	}

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	return store.AddHistory(path)
}

func choiceHistory(_ context.Context, _ *cli.Command) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	hs, err := store.Histories()
	if err != nil {
		return err
	}
	if len(hs) == 0 {
		return errors.New("no history")
	}

	// hs[0] は現在地なので候補から外す。
	candidates := hs[1:]
	labels := make([]string, len(candidates))
	for i, h := range candidates {
		labels[i] = h.Tilde()
	}

	idx, err := selectWithFzf(labels)
	if err != nil {
		return err
	}
	if idx < 0 {
		return nil
	}
	fmt.Println(candidates[idx].Abs())
	return nil
}

func latestHistory(_ context.Context, _ *cli.Command) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	hs, err := store.Histories()
	if err != nil {
		return err
	}
	// hs[0] は現在地。1 つ前が無ければ黙って失敗させ、呼び出し側の fish に
	// 何もさせない。
	if len(hs) < 2 {
		return cli.Exit("", 1)
	}
	fmt.Println(hs[1].Abs())
	return nil
}
