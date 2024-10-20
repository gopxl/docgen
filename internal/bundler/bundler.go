package bundler

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
)

type Bundler struct {
	handlers []Handler
}

func NewBundler() *Bundler {
	return &Bundler{}
}

func (b *Bundler) Add(h Handler) {
	b.handlers = append(b.handlers, h)
}

func (b *Bundler) Compile() (*Bundle, error) {
	bun := Bundle{
		lookup: make(map[string]singleFileHandler),
	}
	for _, h := range b.handlers {
		files, err := h.Files()
		if err != nil {
			return nil, fmt.Errorf("could not cimpile bundle: %w", err)
		}
		for _, file := range files {
			bun.lookup[file] = singleFileHandler{
				h: h,
				f: file,
			}
		}
	}
	return &bun, nil
}

type Bundle struct {
	lookup map[string]singleFileHandler // [dstpath]singleFileHandler
}

type singleFileHandler struct {
	h Handler
	f string
}

func (h singleFileHandler) handle(w io.Writer) error {
	err := h.h.Handle(w, h.f)
	if errors.Is(err, fs.ErrNotExist) {
		return errors.New(fmt.Sprintf("could not find bundled file %s even though it was listed by the handler", h.f))
	}
	return err
}

// Files lists the output file paths in the Bundle.
func (bun *Bundle) Files() []string {
	s := make([]string, len(bun.lookup))
	for dst := range bun.lookup {
		s = append(s, dst)
	}
	sort.Strings(s)
	return s
}

// StoreInDir compiles all files in the Bundle to the given output directory.
func (bun *Bundle) StoreInDir(dir string) error {
	for _, h := range bun.lookup {
		if err := bun.handleAndStoreInDir(h, dir); err != nil {
			return err
		}
	}
	return nil
}

func (bun *Bundle) handleAndStoreInDir(h singleFileHandler, dir string) error {
	p := filepath.Join(dir, h.f)
	err := os.MkdirAll(filepath.Dir(p), 0755)
	if err != nil {
		return fmt.Errorf("could not create directory for output file %v: %w", p, err)
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("could not open output file %v: %w", dir, err)
	}
	defer f.Close()
	return bun.handleAndWrite(h, f)
}

// WriteFileTo compiles the file stored at pth and writes the output to w.
func (bun *Bundle) WriteFileTo(pth string, w io.Writer) error {
	pth = path.Clean(pth)
	if m, ok := bun.lookup[pth]; ok {
		return bun.handleAndWrite(m, w)
	}
	return fs.ErrNotExist
}

func (bun *Bundle) handleAndWrite(h singleFileHandler, w io.Writer) error {
	err := h.handle(w)
	if err != nil {
		return fmt.Errorf("error handling file %s: %w", h.f, err)
	}
	return nil
}
