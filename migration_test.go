package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// legacyDDL は 2019 年当時の gorm v1 が生成した DDL そのまま。
// is_home が bool、path が varchar(255) である点が gorm v2 の期待 (numeric / text)
// と食い違うので、AutoMigrate は初回にテーブルを作り直す。
var legacyDDL = []string{
	`CREATE TABLE IF NOT EXISTS "histories" ("id" integer primary key autoincrement,"created_at" datetime,"updated_at" datetime,"deleted_at" datetime,"path" varchar(255),"is_home" bool );`,
	`CREATE INDEX idx_histories_deleted_at ON "histories"(deleted_at) ;`,
	`CREATE TABLE IF NOT EXISTS "bookmarks" ("id" integer primary key autoincrement,"created_at" datetime,"updated_at" datetime,"deleted_at" datetime,"path" varchar(255),"is_home" bool );`,
	`CREATE INDEX idx_bookmarks_deleted_at ON "bookmarks"(deleted_at) ;`,
	`CREATE TABLE IF NOT EXISTS "window_paths" ("window_id" varchar(255),"id" integer primary key autoincrement,"created_at" datetime,"updated_at" datetime,"deleted_at" datetime,"path" varchar(255),"is_home" bool );`,
	`CREATE INDEX idx_window_paths_deleted_at ON "window_paths"(deleted_at) ;`,
}

// レガシー DB が持っていた時刻表現。gorm v1 + mattn/go-sqlite3 はこの形式で書いていた。
const (
	legacyTime1 = "2024-01-29 14:27:40.537149+09:00" // histories#1
	legacyTime2 = "2024-02-29 10:11:12.000001+09:00" // histories#2 (最新)
	legacyTime3 = "2023-12-01 09:08:07.654321+09:00" // histories#3 (最古)
)

// dump は各テーブルの全カラムを quote() でリテラル化して 1 行 1 レコードで返す。
// quote() は型情報 (テキスト / 整数 / NULL) まで表現に含めるので、
// 「作り直しても中身が 1 バイトも変わらない」ことの検査に使える。
const (
	dumpHistories = `SELECT quote(id) || '|' || quote(created_at) || '|' || quote(updated_at) || '|' ||
		quote(deleted_at) || '|' || quote(path) || '|' || quote(is_home)
		FROM histories WHERE id <= 3 ORDER BY id`
	dumpBookmarks = `SELECT quote(id) || '|' || quote(created_at) || '|' || quote(updated_at) || '|' ||
		quote(deleted_at) || '|' || quote(path) || '|' || quote(is_home)
		FROM bookmarks ORDER BY id`
	dumpWindowPaths = `SELECT quote(window_id) || '|' || quote(id) || '|' || quote(created_at) || '|' ||
		quote(updated_at) || '|' || quote(deleted_at) || '|' || quote(path) || '|' || quote(is_home)
		FROM window_paths ORDER BY id`
)

// openRawDB はキャッシュ DB を Store を通さずに開く。fn の実行後に必ず閉じる。
func openRawDB(t *testing.T, path string, fn func(db *gorm.DB)) {
	t.Helper()

	// openStore と違い MkdirAll をしないので、ここで用意しておく。
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}

	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer func() {
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatalf("db handle: %v", err)
		}
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("close %s: %v", path, err)
		}
	}()

	fn(db)
}

func dumpRows(t *testing.T, db *gorm.DB, query string) []string {
	t.Helper()

	var rows []string
	if err := db.Raw(query).Scan(&rows).Error; err != nil {
		t.Fatalf("dump: %v", err)
	}
	return rows
}

// TestLegacyDatabaseContentIsPreserved は gorm v1 時代の DB を開いても
// 行の中身が 1 バイトも変わらないことを検査する回帰テスト。
//
// AutoMigrate は型の食い違いを埋めるためにテーブルを作り直す (新テーブル作成 →
// INSERT SELECT → DROP → RENAME)。スキーマ文字列が変わるのは想定内なので
// アサートしないが、データが失われたり型が化けたりしてはいけない。
func TestLegacyDatabaseContentIsPreserved(t *testing.T) {
	dbPath := setupEnv(t)

	openRawDB(t, dbPath, func(db *gorm.DB) {
		for _, stmt := range legacyDDL {
			if err := db.Exec(stmt).Error; err != nil {
				t.Fatalf("exec legacy DDL %q: %v", stmt, err)
			}
		}

		inserts := []struct {
			query string
			args  []any
		}{
			{
				`INSERT INTO histories (id, created_at, updated_at, deleted_at, path, is_home) VALUES (?, ?, ?, NULL, ?, ?)`,
				[]any{1, legacyTime1, legacyTime1, "src/legacy-home", 1},
			},
			{
				`INSERT INTO histories (id, created_at, updated_at, deleted_at, path, is_home) VALUES (?, ?, ?, NULL, ?, ?)`,
				[]any{2, legacyTime1, legacyTime2, "/etc/legacy-outside", 0},
			},
			{
				`INSERT INTO histories (id, created_at, updated_at, deleted_at, path, is_home) VALUES (?, ?, ?, NULL, ?, ?)`,
				[]any{3, legacyTime3, legacyTime3, "work/legacy-old", 1},
			},
			{
				`INSERT INTO bookmarks (id, created_at, updated_at, deleted_at, path, is_home) VALUES (?, ?, ?, NULL, ?, ?)`,
				[]any{1, legacyTime1, legacyTime1, "src/legacy-mark", 1},
			},
			{
				`INSERT INTO bookmarks (id, created_at, updated_at, deleted_at, path, is_home) VALUES (?, ?, ?, ?, ?, ?)`,
				[]any{2, legacyTime2, legacyTime2, legacyTime3, "/var/legacy-deleted", 0},
			},
			{
				`INSERT INTO window_paths (window_id, id, created_at, updated_at, deleted_at, path, is_home) VALUES (?, ?, ?, ?, NULL, ?, ?)`,
				[]any{"legacy-window", 1, legacyTime2, legacyTime2, "src/legacy-window", 1},
			},
		}
		for _, ins := range inserts {
			if err := db.Exec(ins.query, ins.args...).Error; err != nil {
				t.Fatalf("insert legacy row: %v", err)
			}
		}
	})

	var (
		beforeHistories   []string
		beforeBookmarks   []string
		beforeWindowPaths []string
		beforeSchema      []string
	)
	openRawDB(t, dbPath, func(db *gorm.DB) {
		beforeHistories = dumpRows(t, db, dumpHistories)
		beforeBookmarks = dumpRows(t, db, dumpBookmarks)
		beforeWindowPaths = dumpRows(t, db, dumpWindowPaths)
		beforeSchema = dumpRows(t, db,
			`SELECT type || '|' || name || '|' || COALESCE(sql, '') FROM sqlite_master ORDER BY type, name`)
	})
	if len(beforeHistories) != 3 || len(beforeBookmarks) != 2 || len(beforeWindowPaths) != 1 {
		t.Fatalf("legacy fixture is wrong: histories=%d bookmarks=%d window_paths=%d",
			len(beforeHistories), len(beforeBookmarks), len(beforeWindowPaths))
	}

	// openStore() が AutoMigrate を走らせる。その上で通常の書き込みも行う。
	store, err := openStore()
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}

	hs, err := store.Histories()
	if err != nil {
		t.Fatalf("Histories: %v", err)
	}
	// updated_at desc がレガシー行の値どおりであること。
	wantOrder := []string{"/etc/legacy-outside", "src/legacy-home", "work/legacy-old"}
	gotOrder := make([]string, len(hs))
	for i, h := range hs {
		gotOrder[i] = h.Path.Path
	}
	if !equalStrings(gotOrder, wantOrder) {
		t.Fatalf("Histories() = %v, want %v", gotOrder, wantOrder)
	}
	// is_home が正しく読めていること。
	if hs[0].IsHome {
		t.Errorf("IsHome = true for %q, want false", hs[0].Path.Path)
	}
	if !hs[1].IsHome {
		t.Errorf("IsHome = false for %q, want true", hs[1].Path.Path)
	}
	if got, want := hs[1].Abs(), homeDir()+"/src/legacy-home"; got != want {
		t.Errorf("Abs() = %q, want %q", got, want)
	}
	// 論理削除済みのブックマークは見えないこと。
	bs, err := store.Bookmarks()
	if err != nil {
		t.Fatalf("Bookmarks: %v", err)
	}
	if len(bs) != 1 {
		t.Fatalf("len(Bookmarks()) = %d, want 1 (the soft-deleted legacy row must stay hidden)", len(bs))
	}
	// レガシーの window_path も読めること。
	wp, err := store.WindowPath("legacy-window")
	if err != nil {
		t.Fatalf("WindowPath: %v", err)
	}
	if wp == nil {
		t.Fatalf("WindowPath(\"legacy-window\") = nil, want a record")
	}

	if err := store.AddHistory("/tmp/cd-utils/after-migration"); err != nil {
		t.Fatalf("AddHistory: %v", err)
	}

	// updated_at はテキストとして保存され、ORDER BY もテキスト比較になる。
	// 新しく書いた行がレガシー行と同じ書式 ("2006-01-02 15:04:05.999999-07:00")
	// でなければ、混在した途端に cd -b の並びが壊れる。
	hs, err = store.Histories()
	if err != nil {
		t.Fatalf("Histories after AddHistory: %v", err)
	}
	wantOrder = append([]string{"/tmp/cd-utils/after-migration"}, wantOrder...)
	gotOrder = make([]string, len(hs))
	for i, h := range hs {
		gotOrder[i] = h.Path.Path
	}
	if !equalStrings(gotOrder, wantOrder) {
		t.Fatalf("Histories() = %v, want %v (new rows must sort above legacy rows)", gotOrder, wantOrder)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var (
		afterHistories   []string
		afterBookmarks   []string
		afterWindowPaths []string
		afterSchema      []string
	)
	openRawDB(t, dbPath, func(db *gorm.DB) {
		afterHistories = dumpRows(t, db, dumpHistories)
		afterBookmarks = dumpRows(t, db, dumpBookmarks)
		afterWindowPaths = dumpRows(t, db, dumpWindowPaths)
		afterSchema = dumpRows(t, db,
			`SELECT type || '|' || name || '|' || COALESCE(sql, '') FROM sqlite_master ORDER BY type, name`)
	})

	for _, tc := range []struct {
		table  string
		before []string
		after  []string
	}{
		{"histories", beforeHistories, afterHistories},
		{"bookmarks", beforeBookmarks, afterBookmarks},
		{"window_paths", beforeWindowPaths, afterWindowPaths},
	} {
		if len(tc.before) != len(tc.after) {
			t.Errorf("%s: %d rows before, %d rows after", tc.table, len(tc.before), len(tc.after))
			continue
		}
		for i := range tc.before {
			if tc.before[i] != tc.after[i] {
				t.Errorf("%s row %d changed:\n before: %s\n  after: %s", tc.table, i, tc.before[i], tc.after[i])
			}
		}
	}

	// スキーマは変わってよい (むしろ変わるのが正しい) が、テーブルは消えていないこと。
	if len(afterSchema) == 0 {
		t.Fatalf("schema is empty after migration")
	}
	t.Logf("schema before AutoMigrate:\n%s", strings.Join(beforeSchema, "\n"))
	t.Logf("schema after AutoMigrate:\n%s", strings.Join(afterSchema, "\n"))
}

// TestOpenStoreIsIdempotentOnLegacyDatabase は 2 回目以降の openStore() が
// さらにテーブルを作り直さない (= スキーマが安定する) ことを確認する。
func TestOpenStoreIsIdempotentOnLegacyDatabase(t *testing.T) {
	dbPath := setupEnv(t)

	openRawDB(t, dbPath, func(db *gorm.DB) {
		for _, stmt := range legacyDDL {
			if err := db.Exec(stmt).Error; err != nil {
				t.Fatalf("exec legacy DDL: %v", err)
			}
		}
		if err := db.Exec(
			`INSERT INTO histories (id, created_at, updated_at, deleted_at, path, is_home) VALUES (1, ?, ?, NULL, 'src/legacy', 1)`,
			legacyTime1, legacyTime1,
		).Error; err != nil {
			t.Fatalf("insert legacy row: %v", err)
		}
	})

	const schemaQuery = `SELECT type || '|' || name || '|' || COALESCE(sql, '') FROM sqlite_master ORDER BY type, name`

	first := openAndClose(t)
	var afterFirst []string
	openRawDB(t, dbPath, func(db *gorm.DB) {
		afterFirst = dumpRows(t, db, schemaQuery)
	})

	second := openAndClose(t)
	var afterSecond []string
	openRawDB(t, dbPath, func(db *gorm.DB) {
		afterSecond = dumpRows(t, db, schemaQuery)
	})

	if first != second {
		t.Fatalf("history count changed unexpectedly: %d then %d", first, second)
	}
	if !equalStrings(afterFirst, afterSecond) {
		t.Fatalf("schema changed on the second open:\n first: %v\nsecond: %v", afterFirst, afterSecond)
	}
}

// openAndClose は openStore → Histories → Close を行い、履歴の件数を返す。
func openAndClose(t *testing.T) int {
	t.Helper()

	store, err := openStore()
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}()

	hs, err := store.Histories()
	if err != nil {
		t.Fatalf("Histories: %v", err)
	}
	return len(hs)
}
