package gitfs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/object"
)

type GitFs struct {
	commit *object.Commit
}

func NewGitFs(commit *object.Commit) *GitFs {
	return &GitFs{commit: commit}
}

func (g *GitFs) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}

	// Try to open the file.
	f, err := g.commit.File(name)
	if errors.Is(err, object.ErrFileNotFound) {
		// Git doesn't store directories explicitly. So we'll do a little
		// more work to be able to open them.
		return g.openDir(name)
	}
	if err != nil {
		return nil, &fs.PathError{
			Op:   "open",
			Path: name,
			Err:  fmt.Errorf("error opening file from commit: %w", err),
		}
	}
	return &GitFile{c: g.commit, f: f}, nil
}

func (g *GitFs) openDir(name string) (fs.File, error) {
	files, err := g.commit.Files()
	if err != nil {
		return nil, &fs.PathError{
			Op:   "open",
			Path: name,
			Err:  fmt.Errorf("error reading files from commit: %w", err),
		}
	}

	name = filepath.Clean(name)
	exists := false
	for {
		file, err := files.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, &fs.PathError{
				Op:   "open",
				Path: name,
				Err:  fmt.Errorf("error iterating over files from commit: %w", err),
			}
		}
		if strings.HasPrefix(filepath.Clean(file.Name), name+"/") {
			exists = true
			break
		}
	}
	if !exists {
		return nil, fs.ErrNotExist
	}

	return &GitDir{
		fs:   g,
		path: name,
	}, nil
}
