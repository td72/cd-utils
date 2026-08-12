package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestToTildePath は絶対パス → (ホーム相対パス, ホーム配下フラグ) の変換と、
// そこから復元される Abs() / Tilde() をまとめて固定する。
//
// /Users/foobar 系のケースは回帰テスト。旧実装は strings.HasPrefix だけで
// 判定していたため、$HOME=/Users/foo のときに /Users/foobar/x を「ホーム配下」と
// 誤判定し、Path="../foobar/x" / IsHome=true として保存、Tilde() が "foobar/x"
// という壊れた文字列を返していた。
func TestToTildePath(t *testing.T) {
	const home = "/Users/foo"

	tests := []struct {
		name       string
		input      string
		wantPath   string
		wantIsHome bool
		wantAbs    string
		wantTilde  string
	}{
		{
			name:       "under home",
			input:      "/Users/foo/src/x",
			wantPath:   "src/x",
			wantIsHome: true,
			wantAbs:    "/Users/foo/src/x",
			wantTilde:  "~/src/x",
		},
		{
			name:       "home itself",
			input:      "/Users/foo",
			wantPath:   ".",
			wantIsHome: true,
			wantAbs:    "/Users/foo",
			wantTilde:  "~",
		},
		{
			name:       "outside home",
			input:      "/etc/nginx",
			wantPath:   "/etc/nginx",
			wantIsHome: false,
			wantAbs:    "/etc/nginx",
			wantTilde:  "/etc/nginx",
		},
		{
			name:       "sibling directory sharing the home prefix",
			input:      "/Users/foobar/x",
			wantPath:   "/Users/foobar/x",
			wantIsHome: false,
			wantAbs:    "/Users/foobar/x",
			wantTilde:  "/Users/foobar/x",
		},
		{
			name:       "sibling directory itself",
			input:      "/Users/foobar",
			wantPath:   "/Users/foobar",
			wantIsHome: false,
			wantAbs:    "/Users/foobar",
			wantTilde:  "/Users/foobar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", home)

			gotPath, gotIsHome := toTildePath(tt.input)
			if gotPath != tt.wantPath || gotIsHome != tt.wantIsHome {
				t.Fatalf("toTildePath(%q) = (%q, %v), want (%q, %v)",
					tt.input, gotPath, gotIsHome, tt.wantPath, tt.wantIsHome)
			}

			p := Path{Path: gotPath, IsHome: gotIsHome}
			if got := p.Abs(); got != tt.wantAbs {
				t.Errorf("Abs() = %q, want %q", got, tt.wantAbs)
			}
			if got := p.Tilde(); got != tt.wantTilde {
				t.Errorf("Tilde() = %q, want %q", got, tt.wantTilde)
			}
		})
	}
}

// TestToTildePathEmptyHome は $HOME を決定できない環境でも panic せず、
// 入力をそのまま「ホーム外」として返すことを固定する (旧実装は panic していた)。
func TestToTildePathEmptyHome(t *testing.T) {
	t.Setenv("HOME", "")

	if got := homeDir(); got != "" {
		t.Fatalf("homeDir() = %q, want %q", got, "")
	}

	const input = "/Users/foo/src/x"
	gotPath, gotIsHome := toTildePath(input)
	if gotPath != input || gotIsHome {
		t.Fatalf("toTildePath(%q) = (%q, %v), want (%q, false)", input, gotPath, gotIsHome, input)
	}

	p := Path{Path: gotPath, IsHome: gotIsHome}
	if got := p.Abs(); got != input {
		t.Errorf("Abs() = %q, want %q", got, input)
	}
	if got := p.Tilde(); got != input {
		t.Errorf("Tilde() = %q, want %q", got, input)
	}
}

// TestHomeDirTrailingSeparator は $HOME に余計な区切りが付いていても
// 正規化されることを確認する。
func TestHomeDirTrailingSeparator(t *testing.T) {
	t.Setenv("HOME", "/Users/foo/")

	if got := homeDir(); got != "/Users/foo" {
		t.Fatalf("homeDir() = %q, want %q", got, "/Users/foo")
	}
	gotPath, gotIsHome := toTildePath("/Users/foo")
	if gotPath != "." || !gotIsHome {
		t.Fatalf(`toTildePath("/Users/foo") = (%q, %v), want (".", true)`, gotPath, gotIsHome)
	}
}

func TestIsDir(t *testing.T) {
	dir := t.TempDir()

	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "existing directory", path: dir, want: true},
		{name: "existing file", path: file, want: false},
		{name: "missing path", path: filepath.Join(dir, "does-not-exist"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDir(tt.path); got != tt.want {
				t.Fatalf("isDir(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// TestIsDirUnreadableParent は os.Stat が NotExist 以外のエラー (権限不足) を
// 返しても panic しないことを確認する回帰テスト。旧実装は nil の FileInfo を
// 参照して panic していた。
func TestIsDirUnreadableParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not honored on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission errors are not reproducible")
	}

	parent := filepath.Join(t.TempDir(), "locked")
	if err := os.MkdirAll(filepath.Join(parent, "child"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chmod(parent, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	// TempDir の後始末が権限不足で失敗しないよう必ず戻す。
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })

	target := filepath.Join(parent, "child")
	if _, err := os.Stat(target); err == nil {
		t.Skipf("%s is still readable; cannot reproduce a permission error", target)
	}
	if got := isDir(target); got {
		t.Fatalf("isDir(%q) = true, want false", target)
	}
}
