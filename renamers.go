package main

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/gosimple/slug"
)

// Renamer rewrites the path to a file.
type Renamer interface {
	Rename(p string) string
}

type nullRenamer struct {
}

func (r *nullRenamer) Rename(p string) string {
	return p
}

type CompositeRenamer struct {
	rs []Renamer
}

func NewCompositeRewriter(rs ...Renamer) *CompositeRenamer {
	return &CompositeRenamer{
		rs: rs,
	}
}

func (c *CompositeRenamer) Rename(p string) string {
	for _, r := range c.rs {
		p = r.Rename(p)
	}
	return p
}

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
