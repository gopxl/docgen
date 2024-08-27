package gitfs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"
)

type GitDir struct {
	fs      *GitFs
	path    string
	files   *object.FileIter
	prevDir string
	eof     bool
}

func (g *GitDir) Stat() (fs.FileInfo, error) {
	return &GitDirInfo{path: g.path, modTime: time.Now()}, nil
}

func (g *GitDir) Read(bytes []byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: g.path, Err: errors.New("is a directory")}
}

func (g *GitDir) Close() error {
	return nil
}

func (g *GitDir) ReadDir(n int) ([]fs.DirEntry, error) {
	if g.eof {
		return nil, io.EOF
	}

	iter, err := g.ensureFiles()
	if err != nil {
		return nil, &fs.PathError{
			Op:   "readdir",
			Path: g.path,
			Err:  fmt.Errorf("error reading files from commit: %w", err),
		}
	}
	var entries []fs.DirEntry
outer:
	for i := 0; i < n || n < 0; i++ {
		// Find the next file that is in the directory.
		for {
			f, err := iter.Next()
			if err != nil {
				if err == io.EOF {
					g.eof = true
					break outer
				}
				return nil, &fs.PathError{
					Op:   "readdir",
					Path: g.path,
					Err:  fmt.Errorf("error iterating over files from commit: %w", err),
				}
			}
			if !strings.HasPrefix(filepath.Clean(f.Name), g.path+"/") {
				continue
			}
			rel, err := filepath.Rel(g.path, f.Name)
			if err != nil {
				return nil, &fs.PathError{
					Op:   "readdir",
					Path: g.path,
					Err:  fmt.Errorf("cannot get relative path of %s to %s: %w", f.Name, g.path, err),
				}
			}
			parts := strings.Split(rel, "/") // todo: use path everywhere instead of filepath.
			if len(parts) == 1 {
				mode, err := f.Mode.ToOSFileMode()
				if err != nil {
					return nil, &fs.PathError{
						Op:   "readdir",
						Path: g.path,
						Err:  fmt.Errorf("error getting file mode for dir entry: %w", err),
					}
				}
				// File is in the directory.
				entries = append(entries, &GitDirEntry{
					fs:   g.fs,
					path: f.Name,
					typ:  mode.Type(),
				})
				break
			} else {
				// File is in a subdirectory.
				p := filepath.Join(g.path, parts[0])
				// Ignore the directory if we've already seen a file in that directory.
				if p == g.prevDir {
					continue
				}
				g.prevDir = p
				entries = append(entries, &GitDirEntry{
					fs:   g.fs,
					path: p,
					typ:  fs.ModeDir,
				})
				break
			}
		}
	}
	return entries, nil
}

func (g *GitDir) ensureFiles() (*object.FileIter, error) {
	if g.files != nil {
		return g.files, nil
	}
	var err error
	g.files, err = g.fs.commit.Files()
	return g.files, err
}

type GitDirInfo struct {
	path    string
	modTime time.Time
}

func (g *GitDirInfo) Name() string {
	return filepath.Base(g.path)
}

func (g *GitDirInfo) IsDir() bool {
	return true
}

func (g *GitDirInfo) Size() int64 {
	return 0
}

func (g *GitDirInfo) Mode() fs.FileMode {
	return fs.ModeDir | 0555
}

func (g *GitDirInfo) ModTime() time.Time {
	return g.modTime
}

func (g *GitDirInfo) Sys() any {
	return g
}
