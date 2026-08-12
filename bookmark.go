package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
)

func bookmarkCommand() *cli.Command {
	return &cli.Command{
		Name:  "bookmark",
		Usage: "cd bookmark utils",
		Commands: []*cli.Command{
			{
				Name:   "list",
				Usage:  "list all bookmarks",
				Action: listBookmark,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "abs",
						Aliases: []string{"a"},
						Usage:   "print absolute path. (default uses tilde)",
					},
				},
			},
			{
				Name:      "add",
				Usage:     "add new bookmark",
				ArgsUsage: "<path>",
				Action:    addBookmark,
			},
			{
				Name:   "choice",
				Usage:  "choice bookmark",
				Action: choiceBookmark,
			},
			{
				Name:   "delete",
				Usage:  "delete bookmark",
				Action: deleteBookmark,
			},
		},
	}
}

func listBookmark(_ context.Context, cmd *cli.Command) error {
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
		// 従来は stdout に "No Bookmark" を出し、fish 側が文字列比較していた。
		// stdout はパスだけを流す約束にしたので stderr へ。
		fmt.Fprintln(os.Stderr, "No bookmark")
		return nil
	}
	abs := cmd.Bool("abs")
	for _, b := range bs {
		if abs {
			fmt.Println(b.Abs())
		} else {
			fmt.Println(b.Tilde())
		}
	}
	return nil
}

// chooseBookmark は fzf でブックマークを選ばせる。
// キャンセルされた場合は (nil, nil) を返す。
func chooseBookmark(store *Store) (*Bookmark, error) {
	bs, err := store.Bookmarks()
	if err != nil {
		return nil, err
	}
	if len(bs) == 0 {
		return nil, errors.New("no bookmark")
	}

	labels := make([]string, len(bs))
	for i, b := range bs {
		labels[i] = b.Tilde()
	}

	idx, err := selectWithFzf(labels)
	if err != nil {
		return nil, err
	}
	if idx < 0 {
		return nil, nil
	}
	return &bs[idx], nil
}

func choiceBookmark(_ context.Context, _ *cli.Command) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	b, err := chooseBookmark(store)
	if err != nil {
		return err
	}
	if b == nil {
		return nil
	}
	fmt.Println(b.Abs())
	return nil
}

func deleteBookmark(_ context.Context, _ *cli.Command) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	b, err := chooseBookmark(store)
	if err != nil {
		return err
	}
	if b == nil {
		return nil
	}

	fmt.Fprintf(os.Stderr, "%s\ndelete? [y/N]: ", b.Tilde())
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	// 従来は空行以外すべてを「削除する」と解釈しており、[y/N] のプロンプトに
	// 反して "n" と答えても削除されていた。
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return store.DeleteBookmark(b)
	}
	fmt.Fprintln(os.Stderr, "canceled")
	return nil
}

func addBookmark(_ context.Context, cmd *cli.Command) error {
	if cmd.NArg() == 0 {
		return errors.New(`"bookmark add" requires a <path> argument`)
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

	created, err := store.AddBookmark(path)
	if err != nil {
		return err
	}
	if !created {
		p, isHome := toTildePath(path)
		fmt.Fprintf(os.Stderr, "%s is already registered\n", Path{Path: p, IsHome: isHome}.Tilde())
	}
	return nil
}
