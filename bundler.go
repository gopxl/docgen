package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

type Context struct {
	bundle  *Bundle
	mapping *Mapping
}

func (c *Context) GetUriSegment(i int) string {
	p := strings.Split(c.mapping.dstPath, "/")
	if i >= len(p) {
		return ""
	}
	return p[i]
}

func (c *Context) ToAbsUrl(file string) *url.URL {
	return c.bundle.rootUrl.JoinPath(file)
}

// RewriteContentUrl rewrites the link so that it points to the new
// location specified in the bundle.
func (c *Context) RewriteContentUrl(link string) (string, error) {
	u, err := url.Parse(link)
	if err != nil {
		return "", fmt.Errorf("cannot parse url %s: %w", link, err)
	}
	if u.IsAbs() {
		return link, nil
	}
	var srcPath string
	if len(u.Path) > 0 && u.Path[0] == '/' {
		// relative to repository root
		srcPath = filepath.Clean(strings.TrimLeft(u.Path, "/"))
	} else {
		// relative to current file
		srcPath = filepath.Join(filepath.Dir(c.mapping.srcPath), u.Path)
	}
	f, ok := c.bundle.srcPaths[c.mapping.srcFs][srcPath]
	if !ok {
		return "", fs.ErrNotExist
	}
	return c.ToAbsUrl(f.dstPath).String(), nil
}

type Bundler struct {
	rules []*MappingRule
}

func NewBundler() *Bundler {
	return &Bundler{}
}

func (b *Bundler) FromFs(fs fs.FS) *MappingRule {
	r := &MappingRule{
		srcFs:    fs,
		compiler: &copyCompiler{},
	}
	b.rules = append(b.rules, r)
	return r
}

func (b *Bundler) Compile(rootUrl *url.URL) (*Bundle, error) {
	bun := Bundle{
		rootUrl:  rootUrl,
		files:    nil,
		srcPaths: make(map[fs.FS]map[string]*Mapping),
		dstPaths: make(map[string]*Mapping),
	}
	for _, r := range b.rules {
		srcPaths, err := r.sourceFiles()
		if err != nil {
			return nil, err
		}
		for _, srcPath := range srcPaths {
			destPath, err := r.DestPath(srcPath)
			if err != nil {
				return nil, err
			}
			bun.files = append(bun.files, Mapping{
				srcFs:    r.srcFs,
				srcPath:  srcPath,
				dstPath:  destPath,
				compiler: r.compiler,
			})
		}
	}
	sort.Slice(bun.files, func(i, j int) bool {
		return bun.files[i].dstPath < bun.files[j].dstPath
	})
	for i, f := range bun.files {
		if _, ok := bun.srcPaths[f.srcFs]; !ok {
			bun.srcPaths[f.srcFs] = make(map[string]*Mapping)
		}
		bun.srcPaths[f.srcFs][f.srcPath] = &bun.files[i]
		bun.dstPaths[f.dstPath] = &bun.files[i]
	}
	return &bun, nil
}

type srcFilesLister func(filesystem fs.FS) ([]string, error)
type srcFileFilter func(file string) bool

type MappingRule struct {
	srcFs          fs.FS
	srcDir         string
	srcFilesLister srcFilesLister
	srcFileFilter  srcFileFilter
	dstDir         string
	compiler       Compiler
}

func (b *MappingRule) TakeFile(file string) *MappingRule {
	b.srcDir = filepath.Dir(file)
	b.srcFilesLister = func(filesystem fs.FS) ([]string, error) {
		_, err := filesystem.Open(file)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("file %s does not exist: %w", file, err)
		}
		return []string{file}, nil
	}
	return b
}

func (b *MappingRule) TakeDir(dir string) *MappingRule {
	return b.TakeGlob(dir, "**/*")
}

func (b *MappingRule) TakeGlob(dir string, glob string) *MappingRule {
	b.srcDir = dir
	b.srcFilesLister = func(filesystem fs.FS) ([]string, error) {
		fullGlob := filepath.Join(dir, glob)
		matches, err := doublestar.Glob(
			filesystem,
			fullGlob,
			doublestar.WithFilesOnly(),
			doublestar.WithFailOnIOErrors(),
			doublestar.WithFailOnPatternNotExist(),
		)
		if err != nil {
			return nil, fmt.Errorf("could not read files with glob %v: %w", fullGlob, err)
		}
		return matches, nil
	}
	return b
}

func (b *MappingRule) Filter(filter srcFileFilter) *MappingRule {
	b.srcFileFilter = filter
	return b
}

func (b *MappingRule) CompileWith(compiler Compiler) *MappingRule {
	b.compiler = compiler
	return b
}

func (b *MappingRule) PutInDir(dir string) {
	b.dstDir = dir
}

func (b *MappingRule) sourceFiles() ([]string, error) {
	files, err := b.srcFilesLister(b.srcFs)
	if err != nil {
		return nil, err
	}
	if b.srcFileFilter == nil {
		return files, nil
	}
	var filtered []string
	for _, file := range files {
		if b.srcFileFilter(file) {
			filtered = append(filtered, file)
		}
	}
	return filtered, nil
}

func (b *MappingRule) DestPath(srcPath string) (string, error) {
	rel, err := filepath.Rel(b.srcDir, srcPath)
	if err != nil {
		return "", fmt.Errorf("could not get relative path of file %v to source directory %v: %w", srcPath, b.srcDir, err)
	}
	return filepath.Join(
		b.dstDir,
		filepath.Dir(rel),
		b.compiler.OutputFileName(filepath.Base(rel)),
	), nil
}

type Compiler interface {
	OutputFileName(oldName string) (newName string)
	Compile(dst io.Writer, src io.Reader, request *Context) error
}

type copyCompiler struct{}

func (c *copyCompiler) OutputFileName(oldName string) (newName string) {
	return oldName
}

func (c *copyCompiler) Compile(dst io.Writer, src io.Reader, request *Context) error {
	_, err := io.Copy(dst, src)
	return err
}

type Bundle struct {
	rootUrl  *url.URL
	files    []Mapping
	srcPaths map[fs.FS]map[string]*Mapping
	dstPaths map[string]*Mapping
}

type Mapping struct {
	srcFs    fs.FS
	srcPath  string
	dstPath  string
	compiler Compiler
}

func (m *Mapping) Open() (fs.File, error) {
	return m.srcFs.Open(m.srcPath)
}

func (bun *Bundle) DestFiles() []string {
	files := make([]string, len(bun.files))
	for _, f := range bun.files {
		files = append(files, f.dstPath)
	}
	return files
}

func (bun *Bundle) CompileAllToDir(dir string) error {
	for i := range bun.files {
		if err := bun.compileFileToDir(&bun.files[i], dir); err != nil {
			return err
		}
	}
	return nil
}

func (bun *Bundle) compileFileToDir(f *Mapping, dir string) error {
	absDstPath := filepath.Join(dir, f.dstPath)

	err := os.MkdirAll(filepath.Dir(absDstPath), 0755)
	if err != nil {
		return fmt.Errorf("could not create directory for output file %v: %w", absDstPath, err)
	}
	dst, err := os.OpenFile(absDstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("could not open output file %v: %w", dir, err)
	}
	defer dst.Close()

	return bun.compileFileToWriter(f, dst)
}

// CompileFileToWriter accepts the destination path pth of a file and writes
// its compiled contents to writer w.
func (bun *Bundle) CompileFileToWriter(pth string, w io.Writer) error {
	pth = path.Clean(pth)
	f, ok := bun.dstPaths[pth]
	if !ok {
		return fs.ErrNotExist
	}
	return bun.compileFileToWriter(f, w)
}

func (bun *Bundle) compileFileToWriter(f *Mapping, w io.Writer) error {
	src, err := f.Open()
	if err != nil {
		return fmt.Errorf("could not open source file %s: %w", f.srcPath, err)
	}
	defer src.Close()
	err = f.compiler.Compile(w, src, &Context{
		mapping: f,
		bundle:  bun,
	})
	if err != nil {
		return fmt.Errorf("could not compile file %v: %w", f.srcPath, err)
	}
	return nil
}
