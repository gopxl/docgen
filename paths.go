package main

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/gosimple/slug"
)

// stripNumberPrefix strips the number in the format "01." from the beginning of the string.
func stripNumberPrefix(str string) string {
	reg, err := regexp.Compile("^[0-9]+\\.\\s*")
	if err != nil {
		panic(fmt.Sprintf("cannot compile regex: %v", err))
	}
	str = reg.ReplaceAllString(str, "")
	return str
}

type PathRewriter struct {
}

func (r *PathRewriter) ModifyPath(p string) string {
	parts := strings.Split(p, "/")

	ext := path.Ext(parts[len(parts)-1])
	if ext == ".md" {
		parts[len(parts)-1] = r.rewritePageFilename(parts[len(parts)-1])
	}

	if len(parts) > 1 {
		parts[0] = r.rewriteSectionDirname(parts[0])
	}

	return strings.Join(parts, "/")
}

func (r *PathRewriter) rewritePageFilename(filename string) string {
	ext := path.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	base = stripNumberPrefix(base)
	base = slug.Make(base)
	return base + ext
}

func (r *PathRewriter) rewriteSectionDirname(dirname string) string {
	dirname = stripNumberPrefix(dirname)
	dirname = slug.Make(dirname)
	return dirname
}
