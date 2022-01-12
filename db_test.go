package main

import (
	"testing"
)

func TestNewWindowPath(t *testing.T) {
	windowId := "aiueo"
	path := "~/test.txt"
	wp := NewWindowPath(windowId, path)
	if wp.WindowId != windowId {
		t.Errorf("%v != %v", wp.WindowId, windowId)
	}
	abs := wp.Abs()
	if abs != path {
		t.Errorf("%v != %v", abs, path)
	}
}

// vim:set fenc=utf-8 ff=unix sw=4 ts=4:
