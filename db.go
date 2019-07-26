package main

import (
	"os"
	"strings"
	"reflect"
	"path/filepath"
	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"
)

const (
	CACHE_ENV_NAME     = "CD_HISTORY_CACHE_PATH"
	DEFAULT_CACHE_NAME = "cd-history.db"
)

var (
	HOME_DIR = os.Getenv("HOME")
)

type PathInterface interface {
	Abs() string
	Tilda() string
	GetPath() string
	GetIsHome() bool
}

type Path struct {
	gorm.Model
	Path string
	IsHome bool
}

func (p Path) Abs() string {
	if p.IsHome {
		return filepath.Join(HOME_DIR, p.Path)
	}
	return p.Path
}

func (p Path) Tilda() string {
	if p.IsHome {
		return filepath.Join("~", p.Path)
	}
	return p.Path
}

func (p Path) GetPath() string {
	return p.Path
}

func (p Path) GetIsHome() bool {
	return p.IsHome
}

func convert_to_tilda_path(path string) (string, bool){
	if strings.HasPrefix(path, HOME_DIR) {
		p, err := filepath.Rel(HOME_DIR, path)
		if err != nil {
			panic(err)
		}
		return p, true
	}
	return path, false
}

type History struct {
	*Path
}

func NewHistory(path string) *History {
	new_path, is_home := convert_to_tilda_path(path)
	p := &Path{Path: new_path, IsHome: is_home}
	return &History{p}
}

type Bookmark struct {
	*Path
}

func NewBookmark(path string) *Bookmark {
	new_path, is_home := convert_to_tilda_path(path)
	p := &Path{Path: new_path, IsHome: is_home}
	return &Bookmark{p}
}


func get_cache_path() string {
	cache_path, ok := os.LookupEnv(CACHE_ENV_NAME)
	if ok {
		return cache_path
	}
	xdg, ok := os.LookupEnv("XDG_CACHE_HOME")
	if !ok {
		xdg = filepath.Join(os.Getenv("HOME"), ".cache")
	}
	return filepath.Join(xdg, DEFAULT_CACHE_NAME)
}

type Info struct {
	DB *gorm.DB
}

func NewInfo() *Info {
	cache_path := get_cache_path()
	db, err := gorm.Open("sqlite3", cache_path)
	if err != nil {
		panic(err)
	}
	db.AutoMigrate(&History{})
	db.AutoMigrate(&Bookmark{})
	return &Info{DB: db}
}

func (i *Info) NotExist(path PathInterface) bool {
	var p  = reflect.ValueOf(&path).Elem().Interface()
	return i.DB.Where(
		"path = ? AND is_home = ?",
		path.GetPath(),
		path.GetIsHome(),
	).First(p).RecordNotFound()
}

