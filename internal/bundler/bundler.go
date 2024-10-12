package bundler

import (
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type Bundler struct {
	rules []*bundleRule
}

func NewBundler() *Bundler {
	return &Bundler{}
}

func (b *Bundler) Add(src Source, opts ...Option) {
	r := &bundleRule{
		src: src,
	}
	for _, opt := range opts {
		opt(r)
	}
	b.rules = append(b.rules, r)
}

func (b *Bundler) Compile(rootUrl *url.URL) (*Bundle, error) {
	bun := Bundle{
		rootUrl: rootUrl,
		files:   nil,
		tagged:  make(map[string]map[string]*Mapping),
		lookup:  make(map[string]*Mapping),
	}
	for _, r := range b.rules {
		files, err := r.src.Files()
		if err != nil {
			return nil, fmt.Errorf("could not cimpile bundle: %w", err)
		}
	fileLoop:
		for _, file := range files {
			for _, f := range r.filters {
				if !f(file) {
					continue fileLoop
				}
			}
			bun.files = append(bun.files, Mapping{
				src:       r.src,
				SrcPath:   file,
				modifiers: r.modifiers,
				storePath: path.Join(r.dstDir, r.modifiers.ModifyPath(file)),
				tag:       r.tag,
			})
		}
	}
	sort.Slice(bun.files, func(i, j int) bool {
		return bun.files[i].storePath < bun.files[j].storePath
	})
	for i, m := range bun.files {
		bun.lookup[m.storePath] = &bun.files[i]
		if m.tag != "" {
			if _, ok := bun.tagged[m.tag]; !ok {
				bun.tagged[m.tag] = make(map[string]*Mapping)
			}
			if _, ok := bun.tagged[m.tag][m.SrcPath]; ok {
				return nil, fmt.Errorf("found multiple files with the same tag and source path: tag: %q path: %q", m.tag, m.SrcPath)
			}
			bun.tagged[m.tag][m.SrcPath] = &bun.files[i]
		}
	}
	return &bun, nil
}

type bundleRule struct {
	src       Source
	filters   []FilterFunc
	modifiers ModifierSlice
	dstDir    string
	tag       string
}

type Option func(r *bundleRule)

type FilterFunc func(path string) bool

func Filter(f FilterFunc) Option {
	return func(r *bundleRule) {
		r.filters = append(r.filters, f)
	}
}

func Pipeline(m ...interface{}) Option {
	return func(r *bundleRule) {
		r.modifiers = append(r.modifiers, m...)
	}
}

func StoreIn(dir string) Option {
	return func(r *bundleRule) {
		r.dstDir = strings.TrimLeft(path.Clean(dir), "/")
	}
}

func Tag(tag string) Option {
	return func(r *bundleRule) {
		r.tag = tag
	}
}

type Bundle struct {
	rootUrl *url.URL
	files   []Mapping
	lookup  map[string]*Mapping            // [dstpath]*Mapping
	tagged  map[string]map[string]*Mapping // [tag][srcpath]*Mapping
}

type Mapping struct {
	src       Source
	SrcPath   string
	modifiers ModifierSlice
	storePath string
	tag       string
}

// Files lists the output file paths in the Bundle.
func (bun *Bundle) Files() []string {
	s := make([]string, len(bun.files))
	for _, m := range bun.files {
		s = append(s, m.storePath)
	}
	return s
}

// StoreInDir compiles all files in the Bundle to the given output directory.
func (bun *Bundle) StoreInDir(dir string) error {
	for i := range bun.files {
		if err := bun.storeMappingInDir(&bun.files[i], dir); err != nil {
			return err
		}
	}
	return nil
}

func (bun *Bundle) storeMappingInDir(m *Mapping, dir string) error {
	storePth := filepath.Join(dir, m.storePath)
	err := os.MkdirAll(filepath.Dir(storePth), 0755)
	if err != nil {
		return fmt.Errorf("could not create directory for output file %v: %w", storePth, err)
	}
	dst, err := os.OpenFile(storePth, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("could not open output file %v: %w", dir, err)
	}
	defer dst.Close()
	return bun.writeMappingTo(m, dst)
}

// WriteFileTo compiles the file stored at pth and writes the output to w.
func (bun *Bundle) WriteFileTo(pth string, w io.Writer) error {
	pth = path.Clean(pth)
	if m, ok := bun.lookup[pth]; ok {
		return bun.writeMappingTo(m, w)
	}
	return fs.ErrNotExist
}

func (bun *Bundle) writeMappingTo(m *Mapping, w io.Writer) error {
	r, err := m.src.Open(m.SrcPath)
	if err != nil {
		return fmt.Errorf("could not open source file %s: %w", m.SrcPath, err)
	}
	defer r.Close()

	return m.modifiers.ModifyContent(r, w, &Context{
		Bundle:  bun,
		Mapping: m,
	})
}
