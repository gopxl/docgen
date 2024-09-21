package main

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/gosimple/slug"
)

type SectionDirectoryRenamer struct {
}

func (s *SectionDirectoryRenamer) Rename(p string) string {
	dir := path.Dir(p)
	if dir == "." {
		return p
	}

	parts := strings.Split(dir, "/")
	r, err := regexp.Compile("^[0-9]+\\.\\s*")
	if err != nil {
		panic(fmt.Sprintf("cannot compile regex: %v", err))
	}
	parts[0] = r.ReplaceAllString(parts[0], "")
	parts[0] = slug.Make(parts[0])
	dir = path.Join(parts...)

	return path.Join(dir, path.Base(p))
}

type PageFileRenamer struct {
}

func (s *PageFileRenamer) Rename(p string) string {
	ext := path.Ext(p)
	file := path.Base(p)
	file = strings.TrimSuffix(file, ext)
	r, err := regexp.Compile("^[0-9]+\\.\\s*")
	if err != nil {
		panic(fmt.Sprintf("cannot compile regex: %v", err))
	}
	file = r.ReplaceAllString(file, "")
	file = slug.Make(file)

	return path.Join(path.Dir(p), file+ext)
}
