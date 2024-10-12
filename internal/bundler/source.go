package bundler

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"
)

type Source interface {
	Files() ([]string, error)
	Open(file string) (io.ReadCloser, error)
}

type FsFileSource struct {
	fs   fs.FS
	path string
}

func NewFsFileSource(fs fs.FS, path string) *FsFileSource {
	return &FsFileSource{
		fs:   fs,
		path: path,
	}
}

func (f *FsFileSource) Files() ([]string, error) {
	return []string{filepath.Base(f.path)}, nil
}

func (f *FsFileSource) Open(file string) (io.ReadCloser, error) {
	return f.fs.Open(f.path)
}

type FsGlobSource struct {
	fs   fs.FS
	dir  string
	glob string
}

func NewFsGlobSource(fs fs.FS, dir, glob string) *FsGlobSource {
	return &FsGlobSource{
		fs:   fs,
		dir:  dir,
		glob: glob,
	}
}

func NewFsDirSource(fs fs.FS, dir string) *FsGlobSource {
	return NewFsGlobSource(fs, dir, "**/*")
}

func (b *FsGlobSource) Files() ([]string, error) {
	fullGlob := filepath.Join(b.dir, b.glob)
	matches, err := doublestar.Glob(
		b.fs,
		fullGlob,
		doublestar.WithFilesOnly(),
		doublestar.WithFailOnIOErrors(),
		doublestar.WithFailOnPatternNotExist(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not read files with glob %v: %w", fullGlob, err)
	}
	for i, m := range matches {
		matches[i], err = filepath.Rel(b.dir, m)
		if err != nil {
			return nil, fmt.Errorf("could not determine the relative path to %s from %s: %w", m, b.dir, err)
		}
	}
	return matches, nil
}

func (b *FsGlobSource) Open(file string) (io.ReadCloser, error) {
	return b.fs.Open(filepath.Join(b.dir, file))
}

type EmptyFileSource struct {
	Path string
}

func (e *EmptyFileSource) Files() ([]string, error) {
	return []string{e.Path}, nil
}

func (e *EmptyFileSource) Open(file string) (io.ReadCloser, error) {
	return &emptyReadCloser{}, nil
}

type emptyReadCloser struct {
}

func (e *emptyReadCloser) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

func (e *emptyReadCloser) Close() error {
	return nil
}
