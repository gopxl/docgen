package gitfs

import (
	"io/fs"
	"path/filepath"
	"time"
)

type GitFileInfo struct {
	filesys *GitFs
	i       int
	size    int64
	modTime time.Time
}

func (g *GitFileInfo) Name() string {
	return filepath.Base(g.info().path)
}

func (g *GitFileInfo) Size() int64 {
	return g.size
}

func (g *GitFileInfo) Mode() fs.FileMode {
	return g.info().mode
}

func (g *GitFileInfo) ModTime() time.Time {
	return g.modTime
}

func (g *GitFileInfo) IsDir() bool {
	return g.info().isDir
}

func (g *GitFileInfo) Sys() any {
	return g
}

func (g *GitFileInfo) info() *fileInfo {
	return &g.filesys.paths[g.i]
}
