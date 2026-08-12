package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// tick は updated_at の進みを確実に観測できるだけの間隔。
const tick = 20 * time.Millisecond

// setupEnv はテスト用の $HOME とキャッシュ DB のパスを設定し、DB のパスを返す。
// 実ユーザーの ~/.cache/cd-history.db に触らないことをここで検査する。
func setupEnv(t *testing.T) string {
	t.Helper()

	base := t.TempDir()
	home := filepath.Join(base, "home")
	dbPath := filepath.Join(base, "cache", "test.db")

	t.Setenv("HOME", home)
	t.Setenv("CD_HISTORY_CACHE_PATH", dbPath)

	if got := cachePath(); got != dbPath {
		t.Fatalf("cachePath() = %q, want %q", got, dbPath)
	}
	return dbPath
}

// newTestStore は使い捨ての DB を開いた Store を返す。
func newTestStore(t *testing.T) *Store {
	t.Helper()

	setupEnv(t)

	store, err := openStore()
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// countRows は論理削除された行も含めた実際の行数を返す。
func countRows(t *testing.T, store *Store, table string) int64 {
	t.Helper()

	var n int64
	if err := store.db.Raw("SELECT count(*) FROM " + table).Scan(&n).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func historyPaths(hs []History) []string {
	out := make([]string, len(hs))
	for i, h := range hs {
		out[i] = h.Abs()
	}
	return out
}

func bookmarkPaths(bs []Bookmark) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Abs()
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestAddHistoryKeepsOneRowPerDirectory は「1 ディレクトリ 1 行」という不変条件の
// 回帰テスト。cd -b (history latest) が hs[0] を現在地とみなす前提を支えているため、
// 2 回目の AddHistory が INSERT に化けると静かに壊れる。
func TestAddHistoryKeepsOneRowPerDirectory(t *testing.T) {
	store := newTestStore(t)

	const dir = "/tmp/cd-utils/one"

	if err := store.AddHistory(dir); err != nil {
		t.Fatalf("AddHistory (1st): %v", err)
	}
	first, err := store.Histories()
	if err != nil {
		t.Fatalf("Histories (1st): %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("len(Histories()) = %d, want 1", len(first))
	}

	time.Sleep(tick)

	if err := store.AddHistory(dir); err != nil {
		t.Fatalf("AddHistory (2nd): %v", err)
	}
	second, err := store.Histories()
	if err != nil {
		t.Fatalf("Histories (2nd): %v", err)
	}

	if len(second) != 1 {
		t.Fatalf("len(Histories()) = %d after re-adding the same path, want 1", len(second))
	}
	if n := countRows(t, store, "histories"); n != 1 {
		t.Fatalf("raw row count = %d, want 1 (a new row was inserted instead of an update)", n)
	}
	if second[0].ID != first[0].ID {
		t.Fatalf("ID = %d, want %d (the existing row must be updated in place)", second[0].ID, first[0].ID)
	}
	if !second[0].UpdatedAt.After(first[0].UpdatedAt) {
		t.Fatalf("UpdatedAt = %v, want later than %v", second[0].UpdatedAt, first[0].UpdatedAt)
	}
	if got := second[0].Abs(); got != dir {
		t.Fatalf("Abs() = %q, want %q", got, dir)
	}
}

// TestHistoriesOrder は Histories() が updated_at desc で返ること、
// 既存パスの再追加で先頭に来ることを確認する。
func TestHistoriesOrder(t *testing.T) {
	store := newTestStore(t)

	dirs := []string{"/tmp/cd-utils/a", "/tmp/cd-utils/b", "/tmp/cd-utils/c"}
	for i, dir := range dirs {
		if i > 0 {
			time.Sleep(tick)
		}
		if err := store.AddHistory(dir); err != nil {
			t.Fatalf("AddHistory(%q): %v", dir, err)
		}
	}

	hs, err := store.Histories()
	if err != nil {
		t.Fatalf("Histories: %v", err)
	}
	want := []string{"/tmp/cd-utils/c", "/tmp/cd-utils/b", "/tmp/cd-utils/a"}
	if got := historyPaths(hs); !equalStrings(got, want) {
		t.Fatalf("Histories() = %v, want %v", got, want)
	}

	time.Sleep(tick)

	if err := store.AddHistory(dirs[0]); err != nil {
		t.Fatalf("AddHistory(%q) again: %v", dirs[0], err)
	}
	hs, err = store.Histories()
	if err != nil {
		t.Fatalf("Histories: %v", err)
	}
	want = []string{"/tmp/cd-utils/a", "/tmp/cd-utils/c", "/tmp/cd-utils/b"}
	if got := historyPaths(hs); !equalStrings(got, want) {
		t.Fatalf("Histories() = %v, want %v", got, want)
	}
}

// TestAddHistoryHomeRelative はホーム配下のパスが相対形で保存され、
// Abs() / Tilde() で元に戻ることを確認する。
func TestAddHistoryHomeRelative(t *testing.T) {
	store := newTestStore(t)

	abs := filepath.Join(homeDir(), "src", "x")
	if err := store.AddHistory(abs); err != nil {
		t.Fatalf("AddHistory: %v", err)
	}

	hs, err := store.Histories()
	if err != nil {
		t.Fatalf("Histories: %v", err)
	}
	if len(hs) != 1 {
		t.Fatalf("len(Histories()) = %d, want 1", len(hs))
	}
	if !hs[0].IsHome {
		t.Errorf("IsHome = false, want true")
	}
	if hs[0].Path.Path != filepath.Join("src", "x") {
		t.Errorf("Path = %q, want %q", hs[0].Path.Path, filepath.Join("src", "x"))
	}
	if got := hs[0].Abs(); got != abs {
		t.Errorf("Abs() = %q, want %q", got, abs)
	}
	if got, want := hs[0].Tilde(), filepath.Join("~", "src", "x"); got != want {
		t.Errorf("Tilde() = %q, want %q", got, want)
	}
}

func TestDeleteHistory(t *testing.T) {
	store := newTestStore(t)

	for _, dir := range []string{"/tmp/cd-utils/a", "/tmp/cd-utils/b"} {
		if err := store.AddHistory(dir); err != nil {
			t.Fatalf("AddHistory(%q): %v", dir, err)
		}
	}

	hs, err := store.Histories()
	if err != nil {
		t.Fatalf("Histories: %v", err)
	}
	if len(hs) != 2 {
		t.Fatalf("len(Histories()) = %d, want 2", len(hs))
	}

	target := hs[0]
	if err := store.DeleteHistory(&target); err != nil {
		t.Fatalf("DeleteHistory: %v", err)
	}

	hs, err = store.Histories()
	if err != nil {
		t.Fatalf("Histories after delete: %v", err)
	}
	if len(hs) != 1 {
		t.Fatalf("len(Histories()) = %d after delete, want 1", len(hs))
	}
	if hs[0].ID == target.ID {
		t.Fatalf("deleted row (ID=%d) is still listed", target.ID)
	}
}

func TestAddBookmark(t *testing.T) {
	store := newTestStore(t)

	const dir = "/tmp/cd-utils/mark"

	created, err := store.AddBookmark(dir)
	if err != nil {
		t.Fatalf("AddBookmark (1st): %v", err)
	}
	if !created {
		t.Fatalf("AddBookmark (1st) = false, want true")
	}

	created, err = store.AddBookmark(dir)
	if err != nil {
		t.Fatalf("AddBookmark (2nd): %v", err)
	}
	if created {
		t.Fatalf("AddBookmark (2nd) = true, want false")
	}

	bs, err := store.Bookmarks()
	if err != nil {
		t.Fatalf("Bookmarks: %v", err)
	}
	if len(bs) != 1 {
		t.Fatalf("len(Bookmarks()) = %d, want 1", len(bs))
	}
	if n := countRows(t, store, "bookmarks"); n != 1 {
		t.Fatalf("raw row count = %d, want 1", n)
	}
}

// TestBookmarksOrder は Bookmarks() が path desc で返ることを確認する。
func TestBookmarksOrder(t *testing.T) {
	store := newTestStore(t)

	// 追加順と path desc 順がずれるように並べる。
	for _, dir := range []string{"/tmp/cd-utils/b", "/tmp/cd-utils/a", "/tmp/cd-utils/c"} {
		if _, err := store.AddBookmark(dir); err != nil {
			t.Fatalf("AddBookmark(%q): %v", dir, err)
		}
	}

	bs, err := store.Bookmarks()
	if err != nil {
		t.Fatalf("Bookmarks: %v", err)
	}
	want := []string{"/tmp/cd-utils/c", "/tmp/cd-utils/b", "/tmp/cd-utils/a"}
	if got := bookmarkPaths(bs); !equalStrings(got, want) {
		t.Fatalf("Bookmarks() = %v, want %v", got, want)
	}
}

func TestDeleteBookmark(t *testing.T) {
	store := newTestStore(t)

	for _, dir := range []string{"/tmp/cd-utils/a", "/tmp/cd-utils/b"} {
		if _, err := store.AddBookmark(dir); err != nil {
			t.Fatalf("AddBookmark(%q): %v", dir, err)
		}
	}

	bs, err := store.Bookmarks()
	if err != nil {
		t.Fatalf("Bookmarks: %v", err)
	}
	if len(bs) != 2 {
		t.Fatalf("len(Bookmarks()) = %d, want 2", len(bs))
	}

	target := bs[0] // "/tmp/cd-utils/b"
	if err := store.DeleteBookmark(&target); err != nil {
		t.Fatalf("DeleteBookmark: %v", err)
	}

	bs, err = store.Bookmarks()
	if err != nil {
		t.Fatalf("Bookmarks after delete: %v", err)
	}
	want := []string{"/tmp/cd-utils/a"}
	if got := bookmarkPaths(bs); !equalStrings(got, want) {
		t.Fatalf("Bookmarks() = %v, want %v", got, want)
	}

	// 論理削除なので実行数は減らない。
	if n := countRows(t, store, "bookmarks"); n != 2 {
		t.Fatalf("raw row count = %d, want 2 (delete must be soft)", n)
	}
}

// TestAddBookmarkAfterDeleteRecreates は論理削除済みのパスを再登録できること
// (= 削除済み行が First に引っかからないこと) を確認する。
func TestAddBookmarkAfterDeleteRecreates(t *testing.T) {
	store := newTestStore(t)

	const dir = "/tmp/cd-utils/mark"

	if _, err := store.AddBookmark(dir); err != nil {
		t.Fatalf("AddBookmark: %v", err)
	}
	bs, err := store.Bookmarks()
	if err != nil {
		t.Fatalf("Bookmarks: %v", err)
	}
	target := bs[0]
	if err := store.DeleteBookmark(&target); err != nil {
		t.Fatalf("DeleteBookmark: %v", err)
	}

	created, err := store.AddBookmark(dir)
	if err != nil {
		t.Fatalf("AddBookmark after delete: %v", err)
	}
	if !created {
		t.Fatalf("AddBookmark after delete = false, want true")
	}

	bs, err = store.Bookmarks()
	if err != nil {
		t.Fatalf("Bookmarks: %v", err)
	}
	if got := bookmarkPaths(bs); !equalStrings(got, []string{dir}) {
		t.Fatalf("Bookmarks() = %v, want %v", got, []string{dir})
	}
}

func TestWindowPathUnknownReturnsNil(t *testing.T) {
	store := newTestStore(t)

	wp, err := store.WindowPath("no-such-window")
	if err != nil {
		t.Fatalf("WindowPath: %v", err)
	}
	if wp != nil {
		t.Fatalf("WindowPath() = %+v, want nil", wp)
	}
}

// TestSaveWindowPathUpsert は同じ window_id への保存が行を増やさず上書きになること、
// 別 window_id と独立していることを確認する。
func TestSaveWindowPathUpsert(t *testing.T) {
	store := newTestStore(t)

	if err := store.SaveWindowPath("w1", "/tmp/cd-utils/first"); err != nil {
		t.Fatalf("SaveWindowPath: %v", err)
	}

	wp, err := store.WindowPath("w1")
	if err != nil {
		t.Fatalf("WindowPath: %v", err)
	}
	if wp == nil {
		t.Fatalf("WindowPath() = nil, want a record")
	}
	if got := wp.Abs(); got != "/tmp/cd-utils/first" {
		t.Fatalf("Abs() = %q, want %q", got, "/tmp/cd-utils/first")
	}
	firstID := wp.ID

	time.Sleep(tick)

	if err := store.SaveWindowPath("w1", "/tmp/cd-utils/second"); err != nil {
		t.Fatalf("SaveWindowPath (update): %v", err)
	}

	wp, err = store.WindowPath("w1")
	if err != nil {
		t.Fatalf("WindowPath after update: %v", err)
	}
	if wp == nil {
		t.Fatalf("WindowPath() = nil after update, want a record")
	}
	if got := wp.Abs(); got != "/tmp/cd-utils/second" {
		t.Fatalf("Abs() = %q, want %q", got, "/tmp/cd-utils/second")
	}
	if wp.ID != firstID {
		t.Fatalf("ID = %d, want %d (the row must be updated in place)", wp.ID, firstID)
	}
	if n := countRows(t, store, "window_paths"); n != 1 {
		t.Fatalf("raw row count = %d, want 1", n)
	}

	// 別ウィンドウは独立していること。
	if err := store.SaveWindowPath("w2", "/tmp/cd-utils/other"); err != nil {
		t.Fatalf("SaveWindowPath(w2): %v", err)
	}
	w1, err := store.WindowPath("w1")
	if err != nil {
		t.Fatalf("WindowPath(w1): %v", err)
	}
	w2, err := store.WindowPath("w2")
	if err != nil {
		t.Fatalf("WindowPath(w2): %v", err)
	}
	if w1 == nil || w2 == nil {
		t.Fatalf("WindowPath() returned nil: w1=%v w2=%v", w1, w2)
	}
	if got := w1.Abs(); got != "/tmp/cd-utils/second" {
		t.Errorf("w1.Abs() = %q, want %q", got, "/tmp/cd-utils/second")
	}
	if got := w2.Abs(); got != "/tmp/cd-utils/other" {
		t.Errorf("w2.Abs() = %q, want %q", got, "/tmp/cd-utils/other")
	}
	if n := countRows(t, store, "window_paths"); n != 2 {
		t.Fatalf("raw row count = %d, want 2", n)
	}
}

// TestSaveWindowPathHomeRelative はホーム配下のパスが is_home フラグ付きで
// 上書きされることを確認する。
func TestSaveWindowPathHomeRelative(t *testing.T) {
	store := newTestStore(t)

	abs := filepath.Join(homeDir(), "work")
	if err := store.SaveWindowPath("w1", "/tmp/cd-utils/outside"); err != nil {
		t.Fatalf("SaveWindowPath: %v", err)
	}
	if err := store.SaveWindowPath("w1", abs); err != nil {
		t.Fatalf("SaveWindowPath (home): %v", err)
	}

	wp, err := store.WindowPath("w1")
	if err != nil {
		t.Fatalf("WindowPath: %v", err)
	}
	if wp == nil {
		t.Fatalf("WindowPath() = nil, want a record")
	}
	if !wp.IsHome {
		t.Errorf("IsHome = false, want true")
	}
	if got := wp.Abs(); got != abs {
		t.Errorf("Abs() = %q, want %q", got, abs)
	}
	if got, want := wp.Tilde(), filepath.Join("~", "work"); got != want {
		t.Errorf("Tilde() = %q, want %q", got, want)
	}
}

// TestOpenStoreCreatesFreshSchema は空のディレクトリからでも 3 テーブルが
// 作られることを確認する。
func TestOpenStoreCreatesFreshSchema(t *testing.T) {
	store := newTestStore(t)

	for _, table := range []string{"histories", "bookmarks", "window_paths"} {
		var name string
		err := store.db.Raw(
			"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", table,
		).Scan(&name).Error
		if err != nil {
			t.Fatalf("query sqlite_master: %v", err)
		}
		if !strings.EqualFold(name, table) {
			t.Fatalf("table %q was not created", table)
		}
	}
}
