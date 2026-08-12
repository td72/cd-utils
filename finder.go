package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// selectWithFzf は候補を fzf に流し、選ばれた行の添字を返す。
// キャンセルされた場合は -1 を返す (エラーではない)。
//
// fzf は UI を /dev/tty に描くので、stdout をキャプチャしても対話できる。
func selectWithFzf(labels []string) (int, error) {
	if len(labels) == 0 {
		return -1, nil
	}
	if _, err := exec.LookPath("fzf"); err != nil {
		return -1, fmt.Errorf("fzf not found in PATH: %w", err)
	}

	cmd := exec.Command("fzf", "--reverse", "--height", "40%")
	cmd.Stdin = strings.NewReader(strings.Join(labels, "\n") + "\n")
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		// fzf は候補なしで 1、Ctrl-C / ESC で 130 を返す。どちらもキャンセル扱い。
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && (exitErr.ExitCode() == 1 || exitErr.ExitCode() == 130) {
			return -1, nil
		}
		return -1, fmt.Errorf("fzf: %w", err)
	}

	selected := strings.TrimRight(string(out), "\n")
	if selected == "" {
		return -1, nil
	}
	for i, label := range labels {
		if label == selected {
			return i, nil
		}
	}
	return -1, nil
}
