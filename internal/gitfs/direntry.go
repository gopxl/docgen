package gitfs

import (
	"io/fs"
	"path/filepath"
)

type GitDirEntry struct {
	filesys *GitFs
	i       int
}

func (g *GitDirEntry) Name() string {
	return filepath.Base(g.info().path)
}

func (g *GitDirEntry) IsDir() bool {
	return g.info().isDir
}

func (g *GitDirEntry) Type() fs.FileMode {
	return g.info().mode.Type()
}

func (g *GitDirEntry) Info() (fs.FileInfo, error) {
	return g.filesys.openPos(g.i).Stat()
}

func (g *GitDirEntry) info() *fileInfo {
	return &g.filesys.paths[g.i]
}
