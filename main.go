package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

const version = "0.1.0"

func rootCommand() *cli.Command {
	return &cli.Command{
		Name:    "cd-utils",
		Usage:   "cd history and bookmark utilities",
		Version: version,
		Commands: []*cli.Command{
			historyCommand(),
			bookmarkCommand(),
			windowPathCommand(),
		},
	}
}

func main() {
	err := rootCommand().Run(context.Background(), os.Args)
	if err == nil {
		return
	}

	// 従来は Run の戻り値を捨てていたため、どんなエラーでも exit 0 になっていた。
	//
	// なお cli v3 の Run は ExitCoder を受け取ると内部で HandleExitCoder →
	// cli.OsExiter を呼ぶので、通常この分岐には到達しない。将来 cli 側の挙動が
	// 変わっても終了コードが失われないようにするための保険として残している。
	// (テストから Run を直接叩く場合は cli.OsExiter を差し替えないとプロセスごと落ちる)
	var exitCoder cli.ExitCoder
	if errors.As(err, &exitCoder) {
		if msg := exitCoder.Error(); msg != "" {
			fmt.Fprintln(os.Stderr, msg)
		}
		os.Exit(exitCoder.ExitCode())
	}
	fmt.Fprintln(os.Stderr, "cd-utils:", err)
	os.Exit(1)
}
