package main

import (
	"path/filepath"
	"strings"
)

func stripNumberDotPrefix(str string) string {
	if len(str) == 0 {
		return str
	}
	if str[0] < '0' || str[0] > '9' {
		return str
	}
	return strings.TrimLeft(str, "0123456789. ")
}

func filepathIsSubdirOf(basepath, subdir string) bool {
	rel, err := filepath.Rel(basepath, subdir)
	if err != nil {
		return false
	}
	return strings.Contains(rel, "../") || strings.Contains(rel, "..\\")
}
