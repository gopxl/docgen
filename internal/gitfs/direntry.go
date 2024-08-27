package gitfs

import (
	"io/fs"
	"path/filepath"
)

type GitDirEntry struct {
	fs   *GitFs
	path string
	typ  fs.FileMode
}

func (g *GitDirEntry) Name() string {
	return filepath.Base(g.path)
}

func (g *GitDirEntry) IsDir() bool {
	return g.typ.IsDir()
}

func (g *GitDirEntry) Type() fs.FileMode {
	return g.typ
}

func (g *GitDirEntry) Info() (fs.FileInfo, error) {
	f, err := g.fs.Open(g.path)
	if err != nil {
		return nil, err
	}
	return f.Stat()
}
