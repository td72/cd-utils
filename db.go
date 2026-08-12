package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	cacheEnvName     = "CD_HISTORY_CACHE_PATH"
	defaultCacheName = "cd-history.db"
)

// homeDir はホームディレクトリを返す。決定できない場合は空文字を返す。
//
// パッケージ変数にキャッシュしないのは、$HOME 未設定の環境で panic していた
// 従来実装を避けるためと、テストから t.Setenv("HOME", ...) で差し替えられる
// ようにするため。
func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return ""
	}
	return filepath.Clean(h)
}

// toTildePath は絶対パスを「ホーム相対パス + ホーム配下フラグ」に分解する。
//
// 従来実装は strings.HasPrefix だけで判定していたため、$HOME=/Users/foo のとき
// /Users/foobar を home 配下と誤判定し、Tilde() が "foobar" という壊れた文字列を
// 返していた。ここではパス区切りの境界まで見て判定する。
func toTildePath(path string) (string, bool) {
	home := homeDir()
	if home == "" {
		return path, false
	}
	if path == home {
		return ".", true
	}
	if !strings.HasPrefix(path, home+string(filepath.Separator)) {
		return path, false
	}
	rel, err := filepath.Rel(home, path)
	if err != nil {
		return path, false
	}
	return rel, true
}

// Path は履歴・ブックマーク・ウィンドウパスに共通する永続化フィールド。
type Path struct {
	gorm.Model
	Path   string
	IsHome bool
}

// Abs は絶対パスを返す。
func (p Path) Abs() string {
	if p.IsHome {
		return filepath.Join(homeDir(), p.Path)
	}
	return p.Path
}

// Tilde はホーム配下を "~" で表した表示用パスを返す。
func (p Path) Tilde() string {
	if p.IsHome {
		return filepath.Join("~", p.Path)
	}
	return p.Path
}

type History struct {
	Path
}

func (History) TableName() string { return "histories" }

type Bookmark struct {
	Path
}

func (Bookmark) TableName() string { return "bookmarks" }

type WindowPath struct {
	WindowID string `gorm:"column:window_id"`
	Path
}

func (WindowPath) TableName() string { return "window_paths" }

// cachePath はキャッシュ DB のパスを解決する。
func cachePath() string {
	if p, ok := os.LookupEnv(cacheEnvName); ok && p != "" {
		return p
	}
	xdg, ok := os.LookupEnv("XDG_CACHE_HOME")
	if !ok || xdg == "" {
		xdg = filepath.Join(homeDir(), ".cache")
	}
	return filepath.Join(xdg, defaultCacheName)
}

// Store はキャッシュ DB へのアクセスをまとめる。
type Store struct {
	db *gorm.DB
}

// openStore はキャッシュ DB を開き、スキーマを整える。
func openStore() (*Store, error) {
	path := cachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}

	// gorm v2 の既定ロガーは stdout に書く。cd-utils.fish は stdout をコマンド
	// 置換で読むので、ログが混ざると cd 先が壊れる。必ず捨てること。
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	// gorm v1 が 2019 年に作った DDL (is_home bool / path varchar(255)) と
	// gorm v2 が期待する型 (numeric / text) が食い違うため、初回はテーブルが
	// 作り直される (新テーブル作成 → INSERT SELECT → DROP → RENAME)。行の中身は
	// そのまま引き継がれる。2 回目以降は no-op。
	if err := db.AutoMigrate(&History{}, &Bookmark{}, &WindowPath{}); err != nil {
		return nil, fmt.Errorf("migrate %s: %w", path, err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Histories は履歴を新しい順に返す。
func (s *Store) Histories() ([]History, error) {
	var hs []History
	if err := s.db.Order("updated_at desc").Find(&hs).Error; err != nil {
		return nil, err
	}
	return hs, nil
}

// AddHistory は履歴を追加する。既に登録済みなら updated_at だけを進める。
//
// 「1 ディレクトリ 1 行」という不変条件が history latest / choice の添字前提
// (hs[0] が現在地) を支えているので、ここで行を増やしてはいけない。
func (s *Store) AddHistory(abs string) error {
	p, isHome := toTildePath(abs)

	var h History
	err := s.db.Where("path = ? AND is_home = ?", p, isHome).First(&h).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		h = History{Path: Path{Path: p, IsHome: isHome}}
		return s.db.Create(&h).Error
	case err != nil:
		return err
	default:
		return s.db.Model(&History{}).Where("id = ?", h.ID).
			Update("updated_at", time.Now()).Error
	}
}

// DeleteHistory は履歴を論理削除する。
func (s *Store) DeleteHistory(h *History) error {
	return s.db.Delete(h).Error
}

// Bookmarks はブックマークをパス降順で返す。
func (s *Store) Bookmarks() ([]Bookmark, error) {
	var bs []Bookmark
	if err := s.db.Order("path desc").Find(&bs).Error; err != nil {
		return nil, err
	}
	return bs, nil
}

// AddBookmark はブックマークを追加する。既に登録済みなら false を返す。
func (s *Store) AddBookmark(abs string) (bool, error) {
	p, isHome := toTildePath(abs)

	var b Bookmark
	err := s.db.Where("path = ? AND is_home = ?", p, isHome).First(&b).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		b = Bookmark{Path: Path{Path: p, IsHome: isHome}}
		return true, s.db.Create(&b).Error
	case err != nil:
		return false, err
	default:
		return false, nil
	}
}

// DeleteBookmark はブックマークを論理削除する。
func (s *Store) DeleteBookmark(b *Bookmark) error {
	return s.db.Delete(b).Error
}

// WindowPath は window_id に対応するパスを返す。未登録なら nil を返す。
func (s *Store) WindowPath(windowID string) (*WindowPath, error) {
	var wp WindowPath
	err := s.db.Where("window_id = ?", windowID).First(&wp).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, nil
	case err != nil:
		return nil, err
	}
	return &wp, nil
}

// SaveWindowPath は window_id に対応するパスを登録または更新する。
func (s *Store) SaveWindowPath(windowID, abs string) error {
	p, isHome := toTildePath(abs)

	wp, err := s.WindowPath(windowID)
	if err != nil {
		return err
	}
	if wp == nil {
		created := WindowPath{WindowID: windowID, Path: Path{Path: p, IsHome: isHome}}
		return s.db.Create(&created).Error
	}
	return s.db.Model(&WindowPath{}).Where("id = ?", wp.ID).
		Updates(map[string]any{
			"path":       p,
			"is_home":    isHome,
			"updated_at": time.Now(),
		}).Error
}
