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
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

type Request struct {
	bundler      *Bundler
	bundle       *Bundle
	rootUrl      *url.URL
	uri          string // path of the file relative to rootUrl.
	srcPath      string // path to the source file.
	bundleDstDir string // path passed to Bundle.PutInDir().
}

func (r *Request) UrlTo(file string) *url.URL {
	return r.rootUrl.JoinPath(file)
}

type Bundler struct {
	bundles []*Bundle
}

func NewBundler() *Bundler {
	return &Bundler{}
}

func (b *Bundler) FromFs(fs fs.FS) *Bundle {
	bundle := &Bundle{
		srcFs:    fs,
		compiler: &copyCompiler{},
	}
	b.bundles = append(b.bundles, bundle)
	return bundle
}

func (b *Bundler) ListGeneratedFiles() ([]string, error) {
	var files []string
	for _, b := range b.bundles {
		sfs, err := b.SourceFiles()
		if err != nil {
			return nil, err
		}
		for _, sf := range sfs {
			df, err := b.DestPath(sf)
			if err != nil {
				return nil, err
			}
			files = append(files, df)
		}
	}
	return files, nil
}

func (b *Bundler) CompileTo(destDir string, rootUrl *url.URL) error {
	for _, bun := range b.bundles {
		srcPaths, err := bun.SourceFiles()
		if err != nil {
			return err
		}
		for _, srcPath := range srcPaths {
			dstPath, err := bun.DestPath(srcPath)
			if err != nil {
				return err
			}
			absDstPath := filepath.Join(destDir, dstPath)

			src, err := bun.srcFs.Open(srcPath)
			if err != nil {
				return fmt.Errorf("could not open file %v: %w", srcPath, err)
			}
			defer src.Close()

			err = os.MkdirAll(filepath.Dir(absDstPath), 0755)
			if err != nil {
				return fmt.Errorf("could not create directory for output file %v: %w", absDstPath, err)
			}
			dst, err := os.OpenFile(absDstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
			if err != nil {
				return fmt.Errorf("could not open output file %v: %w", destDir, err)
			}
			defer dst.Close()

			err = bun.compiler.Compile(dst, src, &Request{
				bundler:      b,
				bundle:       bun,
				rootUrl:      rootUrl,
				uri:          dstPath,
				srcPath:      srcPath,
				bundleDstDir: bun.dstDir,
			})
			if err != nil {
				return fmt.Errorf("could not compile file %v: %w", srcPath, err)
			}
		}
	}
	return nil
}

// todo: document that p is the dstPath.
func (b *Bundler) WriteFile(p string, w io.Writer, rootUrl *url.URL) error {
	p = path.Clean(p)
	for _, bun := range b.bundles {
		matches, err := doublestar.Match(path.Join("/", bun.dstDir, "**/*"), p)
		if err != nil {
			return fmt.Errorf("could not match request url path: %w", err)
		}
		if !matches {
			continue
		}
		srcPaths, err := bun.SourceFiles()
		if err != nil {
			return fmt.Errorf("could not match request url path: %w", err)
		}
		for _, srcPath := range srcPaths {
			dstPath, err := bun.DestPath(srcPath)
			if err != nil {
				return fmt.Errorf("could not determine destination path of file: %w", err)
			}
			if p != path.Clean(path.Join("/", dstPath)) {
				continue
			}
			src, err := bun.srcFs.Open(srcPath)
			if err != nil {
				return fmt.Errorf("could not open source file %v: %w", srcPath, err)
			}
			defer src.Close()
			err = bun.compiler.Compile(w, src, &Request{
				bundler:      b,
				bundle:       bun,
				rootUrl:      rootUrl,
				uri:          dstPath,
				srcPath:      srcPath,
				bundleDstDir: bun.dstDir,
			})
			if err != nil {
				return fmt.Errorf("could not compile file %v: %w", srcPath, err)
			}
			return nil
		}
	}
	return fs.ErrNotExist
}

// RewriteContentUrl maps link l that was found in bundle bun to
// the new absolute url.
func (b *Bundler) RewriteContentUrl(request *Request, l string) (string, error) {
	u, err := url.Parse(l)
	if err != nil {
		return "", fmt.Errorf("cannot parse url %s: %w", l, err)
	}
	// Absolute path.
	if u.IsAbs() {
		return l, nil
	}

	// Relative path.
	l = path.Clean(u.Path)
	var targetSrcPath string
	if len(l) > 0 && l[0] == '/' {
		// Relative to repository root.
		targetSrcPath = filepath.Clean(strings.TrimLeft(l, "/"))
	} else {
		// Relative to current file.
		targetSrcPath = filepath.Join(filepath.Dir(request.srcPath), l)
	}
	if strings.Contains(targetSrcPath, "../") {
		return "", fmt.Errorf("url %s is outside the filesystem: %w", l, err)
	}

	// Find file.
	for _, ob := range b.bundles {
		// Files cannot be relative to files in other filesystems.
		if ob.srcFs != request.bundle.srcFs {
			continue
		}
		if !filepathIsSubdirOf(targetSrcPath, request.bundle.srcDir) {
			continue
		}
		files, err := ob.SourceFiles()
		if err != nil {
			return "", fmt.Errorf("could not read source files of bundle: %w", err)
		}
		for _, f := range files {
			if f != targetSrcPath {
				continue
			}
			dstPath, err := ob.DestPath(f)
			if err != nil {
				return "", fmt.Errorf("cannot get dstPath of file: %w", err)
			}
			return request.rootUrl.JoinPath(dstPath).String(), nil
		}
	}

	// Not found :(
	return "", fmt.Errorf("url %s cannot by found in the filesystem: %w", l, err)
}

type srcFilesLister func(filesystem fs.FS) ([]string, error)
type srcFileFilter func(file string) bool

type Bundle struct {
	srcFs          fs.FS
	srcDir         string
	srcFilesLister srcFilesLister
	srcFileFilter  srcFileFilter
	dstDir         string
	compiler       Compiler
}

func (b *Bundle) TakeFile(file string) *Bundle {
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

func (b *Bundle) TakeDir(dir string) *Bundle {
	return b.TakeGlob(dir, "**/*")
}

func (b *Bundle) TakeGlob(dir string, glob string) *Bundle {
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

func (b *Bundle) Filter(filter srcFileFilter) *Bundle {
	b.srcFileFilter = filter
	return b
}

func (b *Bundle) CompileWith(compiler Compiler) *Bundle {
	b.compiler = compiler
	return b
}

func (b *Bundle) PutInDir(dir string) {
	b.dstDir = dir
}

func (b *Bundle) SourceFiles() ([]string, error) {
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

func (b *Bundle) DestPath(srcPath string) (string, error) {
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
	Compile(dst io.Writer, src io.Reader, request *Request) error
}

type copyCompiler struct{}

func (c *copyCompiler) OutputFileName(oldName string) (newName string) {
	return oldName
}

func (c *copyCompiler) Compile(dst io.Writer, src io.Reader, request *Request) error {
	_, err := io.Copy(dst, src)
	return err
}
