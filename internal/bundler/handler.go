package bundler

import (
	"fmt"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

type Handler interface {
	Files() ([]string, error)
	Handle(w io.Writer, file string) error
}

type FsFileHandler struct {
	fs   fs.FS
	src  string
	dest string
}

func NewFsFileHandler(fs fs.FS, src, dest string) *FsFileHandler {
	return &FsFileHandler{
		fs:   fs,
		src:  filepath.Clean(src),
		dest: path.Clean(dest),
	}
}

func (h *FsFileHandler) Files() ([]string, error) {
	return []string{h.dest}, nil
}

func (h *FsFileHandler) Handle(w io.Writer, file string) error {
	if path.Clean(file) != h.dest {
		return fs.ErrNotExist
	}
	f, err := h.fs.Open(h.src)
	if err != nil {
		return fmt.Errorf("could not open file %s: %w", h.src, err)
	}
	_, err = io.Copy(w, f)
	if err != nil {
		return fmt.Errorf("could not copy file %s to writer: %w", h.src, err)
	}
	return nil
}

type FsGlobHandler struct {
	fs     fs.FS
	srcDir string
	glob   string
	dstDir string
}

func NewFsGlobHandler(fs fs.FS, srcDir, glob, dstDir string) *FsGlobHandler {
	return &FsGlobHandler{
		fs:     fs,
		srcDir: filepath.Clean(srcDir),
		glob:   glob,
		dstDir: path.Clean(dstDir),
	}
}

func NewFsDirHandler(fs fs.FS, srcDir, dstDir string) *FsGlobHandler {
	return NewFsGlobHandler(fs, srcDir, "**/*", dstDir)
}

func (h *FsGlobHandler) Files() ([]string, error) {
	fullGlob := filepath.Join(h.srcDir, h.glob)
	matches, err := doublestar.Glob(
		h.fs,
		fullGlob,
		doublestar.WithFilesOnly(),
		doublestar.WithFailOnIOErrors(),
		doublestar.WithFailOnPatternNotExist(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not read files with glob %v: %w", fullGlob, err)
	}
	for i, m := range matches {
		relPath, err := filepath.Rel(h.srcDir, m)
		if err != nil {
			return nil, fmt.Errorf("could not determine the relative path to %s from %s: %w", m, h.srcDir, err)
		}
		matches[i] = path.Join(h.dstDir, relPath)
	}
	return matches, nil
}

func (h *FsGlobHandler) Handle(w io.Writer, file string) error {
	file = path.Clean(file)
	if h.dstDir != "." {
		if !strings.HasPrefix(file, h.dstDir+"/") {
			return fs.ErrNotExist
		}
		file = strings.TrimPrefix(file, h.dstDir+"/")
	}
	// todo: check if it matches the glob.

	src := filepath.Join(h.srcDir, file)
	f, err := h.fs.Open(src)
	if err != nil {
		return fmt.Errorf("could not open file %s: %w", src, err)
	}
	_, err = io.Copy(w, f)
	if err != nil {
		return fmt.Errorf("could not copy file %s to writer: %w", src, err)
	}
	return nil
}
