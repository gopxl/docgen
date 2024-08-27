package gitfs

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"
)

type GitFile struct {
	c *object.Commit
	f *object.File
	r io.ReadCloser
}

func (g *GitFile) Stat() (fs.FileInfo, error) {
	return &GitFileInfo{
		c: g.c,
		f: g.f,
	}, nil
}

func (g *GitFile) Read(bytes []byte) (int, error) {
	r, err := g.ensureReader()
	if err != nil {
		return 0, err
	}
	return r.Read(bytes)
}

func (g *GitFile) Close() error {
	if g.r == nil {
		return nil
	}
	return g.r.Close()
}

func (g *GitFile) ensureReader() (io.ReadCloser, error) {
	if g.r != nil {
		return g.r, nil
	}
	var err error
	g.r, err = g.f.Reader()
	if err != nil {
		return nil, fmt.Errorf("cannot get reader from Git file: %w", err)
	}
	return g.r, nil
}

type GitFileInfo struct {
	f *object.File
	c *object.Commit
}

func (g *GitFileInfo) Name() string {
	return filepath.Base(g.f.Name)
}

func (g *GitFileInfo) Size() int64 {
	return g.f.Size
}

func (g *GitFileInfo) Mode() fs.FileMode {
	mode, err := g.f.Mode.ToOSFileMode()
	if err != nil {
		return 0 // ignore
	}
	return mode
}

func (g *GitFileInfo) ModTime() time.Time {
	return g.c.Author.When
}

func (g *GitFileInfo) IsDir() bool {
	return false
}

func (g *GitFileInfo) Sys() any {
	return g.f
}
