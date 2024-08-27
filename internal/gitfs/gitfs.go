package gitfs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/object"
)

type GitFs struct {
	commit *object.Commit
	paths  []fileInfo
}

type fileInfo struct {
	path  string
	isDir bool
	mode  fs.FileMode
}

func NewGitFs(commit *object.Commit) (*GitFs, error) {
	files, err := commit.Files()
	if err != nil {
		return nil, fmt.Errorf("error reading paths from commit: %w", err)
	}
	var infos []fileInfo
	dirs := make(map[string]struct{})
	err = files.ForEach(func(f *object.File) error {
		mode, err := f.Mode.ToOSFileMode()
		if err != nil {
			return fmt.Errorf("could not get mode from file %s: %w", f.Name, err)
		}
		infos = append(infos, fileInfo{
			path:  f.Name,
			isDir: false,
			mode:  mode,
		})
		dir := filepath.Dir(f.Name)
		for dir != "." {
			if _, ok := dirs[dir]; ok {
				break
			}
			infos = append(infos, fileInfo{
				path:  dir,
				isDir: true,
				mode:  fs.ModeDir | 0555,
			})
			dirs[dir] = struct{}{}
			dir = filepath.Dir(dir)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error iterating over paths from commit: %w", err)
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].path < infos[j].path
	})
	return &GitFs{
		commit: commit,
		paths:  infos,
	}, nil
}

func (g *GitFs) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}

	name = filepath.Clean(name)
	i, ok := slices.BinarySearchFunc(g.paths, name, func(info fileInfo, pth string) int {
		return strings.Compare(info.path, pth)
	})
	if !ok {
		return nil, fs.ErrNotExist
	}
	return g.openPos(i), nil
}

func (g *GitFs) openPos(i int) fs.File {
	return &GitFile{
		filesys: g,
		i:       i,
		pos:     i + 1,
	}
}

type GitFile struct {
	filesys *GitFs
	i       int
	f       *object.File
	r       io.ReadCloser
	pos     int // iterator position inside directory
}

func (g *GitFile) Stat() (fs.FileInfo, error) {
	if g.info().isDir {
		return &GitFileInfo{
			filesys: g.filesys,
			i:       g.i,
			size:    0,
			modTime: g.filesys.commit.Author.When, // this is incorrect, but not currently used.
		}, nil
	}

	err := g.ensureFile()
	if err != nil {
		return nil, &fs.PathError{
			Op:   "stat",
			Path: g.info().path,
			Err:  err,
		}
	}

	return &GitFileInfo{
		filesys: g.filesys,
		i:       g.i,
		size:    g.f.Size,
		modTime: g.filesys.commit.Author.When, // this is incorrect, but not currently used.
	}, nil
}

func (g *GitFile) Read(bytes []byte) (n int, err error) {
	info := g.info()
	defer func() {
		if err != nil && err != io.EOF {
			err = &fs.PathError{
				Op:   "read",
				Path: info.path,
				Err:  err,
			}
		}
	}()

	if info.isDir {
		err = errors.New("is a directory")
		return
	}

	err = g.ensureFile()
	if err != nil {
		return
	}
	if g.r == nil {
		g.r, err = g.f.Reader()
		if err != nil {
			err = fmt.Errorf("cannot get reader from Git file: %w", err)
			return
		}
	}
	return g.r.Read(bytes)
}

func (g *GitFile) Close() error {
	if g.r == nil {
		return nil
	}
	return g.r.Close()
}

func (g *GitFile) ReadDir(n int) ([]fs.DirEntry, error) {
	info := g.info()
	if !info.isDir {
		return nil, &fs.PathError{
			Op:   "readdir",
			Path: info.path,
			Err:  errors.New("is not a directory"),
		}
	}

	var c int
	if n < 0 {
		c = len(g.filesys.paths)
	} else {
		c = min(g.pos+n, len(g.filesys.paths))
	}
	var entries []fs.DirEntry
	for ; g.pos < c; g.pos++ {
		inf := g.filesys.paths[g.pos]
		prefix := info.path + "/"
		if !strings.HasPrefix(inf.path, prefix) {
			// Done
			if len(entries) == 0 {
				return nil, io.EOF
			}
			return entries, nil
		}
		// Skip files in subdirectories.
		if strings.Count(inf.path[len(prefix):], "/") > 0 {
			continue
		}
		entries = append(entries, &GitDirEntry{
			filesys: g.filesys,
			i:       g.pos,
		})
	}
	if len(entries) == 0 && g.pos == len(g.filesys.paths) {
		return nil, io.EOF
	}
	return entries, nil
}

func (g *GitFile) info() *fileInfo {
	return &g.filesys.paths[g.i]
}

func (g *GitFile) ensureFile() error {
	if g.f != nil {
		return nil
	}
	var err error
	g.f, err = g.filesys.commit.File(g.info().path)
	if err != nil {
		return fmt.Errorf("cannot open Git file: %w", err)
	}
	return nil
}
