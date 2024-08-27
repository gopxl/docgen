package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

type MenuItem struct {
	Title string
	Path  string
	IsDir bool
	Items []MenuItem
}

func NewMenuFromFs(filesystem fs.FS) ([]MenuItem, error) {
	return menuEntries(filesystem, ".")
}

func menuEntries(filesystem fs.FS, dir string) ([]MenuItem, error) {
	entries, err := fs.ReadDir(filesystem, dir)
	if err != nil {
		return nil, fmt.Errorf("could not read files in directory %s: %w", dir, err)
	}
	var items []MenuItem
	for _, e := range entries {
		var item MenuItem
		if e.IsDir() {
			p := filepath.Join(dir, e.Name())
			sub, err := menuEntries(filesystem, p)
			if err != nil {
				return nil, err
			}

			item = MenuItem{
				Title: stripNumberDotPrefix(e.Name()),
				Path:  p,
				IsDir: true,
				Items: sub,
			}
		} else {
			if filepath.Ext(e.Name()) != ".md" {
				continue
			}
			item = MenuItem{
				Title: stripNumberDotPrefix(strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))),
				Path:  filepath.Join(dir, (&MarkdownCompiler{}).OutputFileName(e.Name())),
				IsDir: false,
				Items: nil,
			}
		}
		items = append(items, item)
	}
	return items, nil
}
