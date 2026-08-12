package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/urfave/cli/v3"
)

func windowPathCommand() *cli.Command {
	return &cli.Command{
		Name:  "window_path",
		Usage: "add and get window_path",
		Commands: []*cli.Command{
			{
				Name:   "update",
				Usage:  "update path and window_id",
				Action: updateWindowPath,
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "window_id", Aliases: []string{"i"}},
					&cli.StringFlag{Name: "path", Aliases: []string{"p"}},
				},
			},
			{
				Name:   "latest",
				Usage:  "get latest path on current window",
				Action: latestWindowPath,
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "window_id", Aliases: []string{"i"}},
				},
			},
		},
	}
}

func updateWindowPath(_ context.Context, cmd *cli.Command) error {
	windowID := cmd.String("window_id")
	if windowID == "" {
		return errors.New("--window_id is required")
	}
	raw := cmd.String("path")
	if raw == "" {
		return errors.New("--path is required")
	}
	// 従来は絶対パス化も存在確認もしておらず、相対パスがそのまま保存されていた。
	path, err := filepath.Abs(raw)
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

	return store.SaveWindowPath(windowID, path)
}

func latestWindowPath(_ context.Context, cmd *cli.Command) error {
	windowID := cmd.String("window_id")
	if windowID == "" {
		return errors.New("--window_id is required")
	}

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	wp, err := store.WindowPath(windowID)
	if err != nil {
		return err
	}
	if wp == nil {
		return cli.Exit("", 1)
	}
	fmt.Println(wp.Abs())
	return nil
}
