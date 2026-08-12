package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/urfave/cli/v3"
)

// cliResult は rootCommand().Run() の結果一式。
type cliResult struct {
	err       error
	exitCodes []int
	stdout    string
}

// runCLI は rootCommand() を直接叩く。
//
// urfave/cli v3 の Run は ExitCoder を受け取ると HandleExitCoder 経由で
// cli.OsExiter (既定は os.Exit) を呼ぶため、差し替えないとテストプロセスごと
// 終了してしまう。ここで捕まえて終了コードを記録する。
func runCLI(t *testing.T, args ...string) cliResult {
	t.Helper()

	var codes []int

	origExiter := cli.OsExiter
	origErrWriter := cli.ErrWriter
	cli.OsExiter = func(code int) { codes = append(codes, code) }
	cli.ErrWriter = io.Discard
	t.Cleanup(func() {
		cli.OsExiter = origExiter
		cli.ErrWriter = origErrWriter
	})

	cmd := rootCommand()
	cmd.Writer = io.Discard
	cmd.ErrWriter = io.Discard

	var err error
	stdout := captureStdout(t, func() {
		err = cmd.Run(context.Background(), append([]string{"cd-utils"}, args...))
	})

	return cliResult{err: err, exitCodes: codes, stdout: stdout}
}

// captureStdout は fn の実行中に os.Stdout へ書かれた内容を返す。
// コマンドの Action は fmt.Println で直接 os.Stdout に書くため、差し替えが必要。
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "stdout-*")
	if err != nil {
		t.Fatalf("create temp stdout: %v", err)
	}
	defer f.Close()

	orig := os.Stdout
	os.Stdout = f
	defer func() { os.Stdout = orig }()

	fn()

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("seek: %v", err)
	}
	out, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	return string(out)
}

// exitCodeOf は err が cli.ExitCoder ならその終了コードを返す。
func exitCodeOf(t *testing.T, err error) int {
	t.Helper()

	var ec cli.ExitCoder
	if !errors.As(err, &ec) {
		t.Fatalf("error %v (%T) does not implement cli.ExitCoder", err, err)
	}
	return ec.ExitCode()
}

func TestHistoryAddRequiresPath(t *testing.T) {
	setupEnv(t)

	res := runCLI(t, "history", "add")
	if res.err == nil {
		t.Fatalf("`history add` without an argument succeeded, want an error")
	}
	if !strings.Contains(res.err.Error(), "<path>") {
		t.Errorf("error = %q, want it to mention the missing <path> argument", res.err)
	}
}

func TestHistoryAddRejectsMissingPath(t *testing.T) {
	setupEnv(t)

	missing := filepath.Join(t.TempDir(), "does-not-exist")
	res := runCLI(t, "history", "add", missing)
	if res.err == nil {
		t.Fatalf("`history add %s` succeeded, want an error", missing)
	}
	if !strings.Contains(res.err.Error(), missing) {
		t.Errorf("error = %q, want it to mention %q", res.err, missing)
	}
}

func TestHistoryAddRejectsFile(t *testing.T) {
	setupEnv(t)

	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	res := runCLI(t, "history", "add", file)
	if res.err == nil {
		t.Fatalf("`history add %s` succeeded, want an error", file)
	}
}

func TestBookmarkAddRequiresPath(t *testing.T) {
	setupEnv(t)

	res := runCLI(t, "bookmark", "add")
	if res.err == nil {
		t.Fatalf("`bookmark add` without an argument succeeded, want an error")
	}
}

// TestHistoryLatestFailsWithoutPrevious は履歴が 2 件未満のときに終了コード 1 で
// 失敗することを固定する。fish 側はこの終了コードだけを見て cd をやめる。
func TestHistoryLatestFailsWithoutPrevious(t *testing.T) {
	dirs := makeDirs(t, 2)

	t.Run("no history", func(t *testing.T) {
		setupEnv(t)

		res := runCLI(t, "history", "latest")
		if res.err == nil {
			t.Fatalf("`history latest` succeeded with an empty history, want exit code 1")
		}
		if code := exitCodeOf(t, res.err); code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		if res.stdout != "" {
			t.Errorf("stdout = %q, want empty", res.stdout)
		}
	})

	t.Run("single entry", func(t *testing.T) {
		setupEnv(t)

		if res := runCLI(t, "history", "add", dirs[0]); res.err != nil {
			t.Fatalf("`history add`: %v", res.err)
		}

		res := runCLI(t, "history", "latest")
		if res.err == nil {
			t.Fatalf("`history latest` succeeded with a single entry, want exit code 1")
		}
		if code := exitCodeOf(t, res.err); code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
	})
}

// TestHistoryLatestPrintsPreviousDirectory は cd -b の本体。
// hs[0] が現在地、hs[1] が直前ディレクトリという添字の前提を固定する。
func TestHistoryLatestPrintsPreviousDirectory(t *testing.T) {
	setupEnv(t)

	dirs := makeDirs(t, 2)

	if res := runCLI(t, "history", "add", dirs[0]); res.err != nil {
		t.Fatalf("`history add %s`: %v", dirs[0], res.err)
	}
	time.Sleep(tick)
	if res := runCLI(t, "history", "add", dirs[1]); res.err != nil {
		t.Fatalf("`history add %s`: %v", dirs[1], res.err)
	}

	res := runCLI(t, "history", "latest")
	if res.err != nil {
		t.Fatalf("`history latest`: %v", res.err)
	}
	if got, want := strings.TrimSpace(res.stdout), dirs[0]; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestHistoryListPrintsTildeAndAbs(t *testing.T) {
	setupEnv(t)

	home := homeDir()
	dir := filepath.Join(home, "src", "x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if res := runCLI(t, "history", "add", dir); res.err != nil {
		t.Fatalf("`history add`: %v", res.err)
	}

	res := runCLI(t, "history", "list")
	if res.err != nil {
		t.Fatalf("`history list`: %v", res.err)
	}
	if got, want := strings.TrimSpace(res.stdout), filepath.Join("~", "src", "x"); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	res = runCLI(t, "history", "list", "--abs")
	if res.err != nil {
		t.Fatalf("`history list --abs`: %v", res.err)
	}
	if got := strings.TrimSpace(res.stdout); got != dir {
		t.Errorf("stdout = %q, want %q", got, dir)
	}
}

// TestHistoryUpdateRemovesMissingDirectories は `history update` が
// 消えたディレクトリを履歴から落とすことを確認する。
func TestHistoryUpdateRemovesMissingDirectories(t *testing.T) {
	setupEnv(t)

	dirs := makeDirs(t, 2)
	for _, dir := range dirs {
		if res := runCLI(t, "history", "add", dir); res.err != nil {
			t.Fatalf("`history add %s`: %v", dir, res.err)
		}
	}
	if err := os.RemoveAll(dirs[0]); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if res := runCLI(t, "history", "update"); res.err != nil {
		t.Fatalf("`history update`: %v", res.err)
	}

	res := runCLI(t, "history", "list", "--abs")
	if res.err != nil {
		t.Fatalf("`history list --abs`: %v", res.err)
	}
	got := strings.Fields(res.stdout)
	if !equalStrings(got, []string{dirs[1]}) {
		t.Fatalf("history = %v, want %v", got, []string{dirs[1]})
	}
}

func TestWindowPathCLI(t *testing.T) {
	setupEnv(t)

	dirs := makeDirs(t, 2)

	t.Run("latest requires window_id", func(t *testing.T) {
		res := runCLI(t, "window_path", "latest")
		if res.err == nil {
			t.Fatalf("`window_path latest` without --window_id succeeded, want an error")
		}
	})

	t.Run("latest of an unknown window exits 1", func(t *testing.T) {
		res := runCLI(t, "window_path", "latest", "--window_id", "unknown")
		if res.err == nil {
			t.Fatalf("`window_path latest` for an unknown window succeeded, want exit code 1")
		}
		if code := exitCodeOf(t, res.err); code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
	})

	t.Run("update then latest", func(t *testing.T) {
		if res := runCLI(t, "window_path", "update", "--window_id", "w1", "--path", dirs[0]); res.err != nil {
			t.Fatalf("`window_path update`: %v", res.err)
		}
		res := runCLI(t, "window_path", "latest", "--window_id", "w1")
		if res.err != nil {
			t.Fatalf("`window_path latest`: %v", res.err)
		}
		if got := strings.TrimSpace(res.stdout); got != dirs[0] {
			t.Fatalf("stdout = %q, want %q", got, dirs[0])
		}

		if res := runCLI(t, "window_path", "update", "--window_id", "w1", "--path", dirs[1]); res.err != nil {
			t.Fatalf("`window_path update` (2nd): %v", res.err)
		}
		res = runCLI(t, "window_path", "latest", "--window_id", "w1")
		if res.err != nil {
			t.Fatalf("`window_path latest` (2nd): %v", res.err)
		}
		if got := strings.TrimSpace(res.stdout); got != dirs[1] {
			t.Fatalf("stdout = %q, want %q", got, dirs[1])
		}
	})

	t.Run("update rejects a missing path", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "nope")
		res := runCLI(t, "window_path", "update", "--window_id", "w1", "--path", missing)
		if res.err == nil {
			t.Fatalf("`window_path update` with a missing path succeeded, want an error")
		}
	})
}

// makeDirs は実在する n 個のディレクトリを作って絶対パスを返す。
func makeDirs(t *testing.T, n int) []string {
	t.Helper()

	base := t.TempDir()
	dirs := make([]string, n)
	for i := range dirs {
		dir := filepath.Join(base, string(rune('a'+i)))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		dirs[i] = dir
	}
	return dirs
}
