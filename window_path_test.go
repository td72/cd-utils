package main

import (
	"flag"
	"fmt"
	"testing"

	"github.com/urfave/cli"
)

func TestUpdateWindowPath(t *testing.T) {
	App = cli.NewApp()
	f := flag.NewFlagSet("window_path", flag.ContinueOnError)
	f.Set("window_path", "aiueo")
	fmt.Println(f.Args())
	c := cli.NewContext(App, f, nil)
	fmt.Println("c.Args().Get(0)")
	fmt.Println(c.Args().Get(0))
}

// vim:set fenc=utf-8 ff=unix sw=4 ts=4:
