package main

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/url"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gopxl/docgen/internal/markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"
)

const templateDir = "resources/views"
const layoutFile = "layout.gohtml"
const redirectFile = "redirect.gohtml"

type DocsHandler struct {
	config     *Config
	templateFs fs.FS
	template   *template.Template
	versions   []*docsVersion
	redirects  map[string]*redirect
}

type docsVersion struct {
	name      string
	fs        fs.FS
	menu      []MenuItem
	srcLookup map[string]*docsFile
	dstLookup map[string]*docsFile
}

type docsFile struct {
	version *docsVersion
	srcPath string
	dstPath string
}

type redirect struct {
	path       string
	redirectTo *docsFile
}

func NewDocsHandler(templateFs fs.FS, config *Config) (*DocsHandler, error) {
	versions, err := GetDocVersions(config)
	if err != nil {
		log.Fatalf("could not determine publishable versions: %v", err)
	}

	h := &DocsHandler{
		config:     config,
		templateFs: templateFs,
		redirects:  make(map[string]*redirect),
	}

	if err := h.loadTemplates(); err != nil {
		return nil, err
	}

	for _, v := range versions {
		var docs docsVersion
		docs.name = v.Name
		docs.srcLookup = make(map[string]*docsFile)
		docs.dstLookup = make(map[string]*docsFile)

		docs.fs, err = fs.Sub(v.FS, config.docsDir)
		if err != nil {
			return nil, fmt.Errorf("could not open the %s documentation subdirectory: %w", config.docsDir, err)
		}

		docs.menu, err = NewMenuFromFs(docs.fs)

		err = fs.WalkDir(docs.fs, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			dstPath := (&PathRewriter{}).ModifyPath(path, false)
			f := &docsFile{
				version: &docs,
				srcPath: path,
				dstPath: dstPath,
			}
			docs.srcLookup[path] = f
			docs.dstLookup[dstPath] = f
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("error traversing docs directory %s: %w", config.docsDir, err)
		}

		h.versions = append(h.versions, &docs)

		// Redirect from site root to default version.
		if v.IsDefault && len(docs.menu) > 0 {
			r := &redirect{
				path:       "index.html",
				redirectTo: docs.srcLookup[docs.menu[0].Items[0].Path],
			}
			h.redirects[r.path] = r
		}
		// Redirect from version root to first section.
		if len(docs.menu) > 0 {
			r := &redirect{
				path:       path.Join(v.Name, "index.html"),
				redirectTo: docs.srcLookup[docs.menu[0].Items[0].Path],
			}
			h.redirects[r.path] = r
		}
		// Redirect from each section root to first page in section.
		for _, item := range docs.menu {
			if !item.IsDir {
				continue
			}
			r := &redirect{
				path:       path.Join(v.Name, (&PathRewriter{}).ModifyPath(item.Path, true), "index.html"),
				redirectTo: docs.srcLookup[item.Items[0].Path],
			}
			h.redirects[r.path] = r
		}
	}

	return h, nil
}

func (h *DocsHandler) Files() ([]string, error) {
	var files []string
	for _, v := range h.versions {
		for f := range v.dstLookup {
			files = append(files, path.Join(v.name, f))
		}
	}
	for _, r := range h.redirects {
		files = append(files, r.path)
	}
	slices.Sort(files)
	slices.Compact(files)
	return files, nil
}

func (h *DocsHandler) Handle(w io.Writer, file string) error {
	err := h.handleFile(w, file)
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return h.handleRedirect(w, file)
}

func (h *DocsHandler) handleFile(w io.Writer, file string) error {
	segments := strings.Split(path.Clean(file), "/")
	if len(segments) == 0 {
		return fs.ErrNotExist
	}
	version := segments[0]
	file = path.Join(segments[1:]...)

	var v *docsVersion
	for i, v2 := range h.versions {
		if v2.name == version {
			v = h.versions[i]
			break
		}
	}
	if v == nil {
		return fs.ErrNotExist
	}
	info, ok := v.dstLookup[file]
	if !ok {
		return fs.ErrNotExist
	}

	switch filepath.Ext(info.srcPath) {
	case ".md":
		return h.handleMarkdown(w, v, info)
	default:
		return h.handleRawFile(w, v, info)
	}
}

func (h *DocsHandler) handleRedirect(w io.Writer, file string) error {
	r, ok := h.redirects[path.Clean(file)]
	if !ok {
		return fs.ErrNotExist
	}

	viewData := struct {
		RedirectUrl string
	}{
		RedirectUrl: h.fileUrl(r.redirectTo).String(),
	}
	if err := h.template.ExecuteTemplate(w, redirectFile, viewData); err != nil {
		return fmt.Errorf("could not render the layout: %v", err)
	}
	return nil
}

func (h *DocsHandler) handleMarkdown(w io.Writer, v *docsVersion, info *docsFile) error {
	f, err := v.fs.Open(info.srcPath)
	if err != nil {
		return fmt.Errorf("could not open file %s: %w", info.srcPath, err)
	}
	defer f.Close()

	// Render markdown.
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(
				util.Prioritized(markdown.NewAbsoluteLinkTargetBlankTransformer(), 1),
				util.Prioritized(markdown.NewUrlTransformer(func(url string) string {
					rewritten, err := h.rewriteContentUrl(v, info, url)
					if err != nil {
						// Ignore error and return original url.
						return url
					}
					return rewritten
				}), 1),
			),
		),
	)
	mdBuf, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("could not read from source: %w", err)
	}
	var buf bytes.Buffer
	if err := md.Convert(mdBuf, &buf); err != nil {
		return fmt.Errorf("could not convert Markdown: %w", err)
	}

	// Render layout.
	if err := h.renderLayout(w, v, info, buf.String()); err != nil {
		return fmt.Errorf("could not render page: %w", err)
	}

	return nil
}

func (h *DocsHandler) handleRawFile(w io.Writer, v *docsVersion, info *docsFile) error {
	f, err := v.fs.Open(info.srcPath)
	if err != nil {
		return fmt.Errorf("could not open file %s: %w", info.srcPath, err)
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	if err != nil {
		return fmt.Errorf("could not copy file %s to writer: %w", info.srcPath, err)
	}
	return nil
}

func (h *DocsHandler) renderLayout(w io.Writer, v *docsVersion, info *docsFile, html string) error {
	title := stripNumberPrefix(strings.TrimSuffix(filepath.Base(info.srcPath), filepath.Ext(info.srcPath)))

	githubUrl, err := h.githubUrl(info.srcPath)
	if err != nil {
		return err
	}

	versions, err := h.versionsViewData(v, info)
	if err != nil {
		return err
	}

	menu, err := h.menuViewData(v, info)
	if err != nil {
		return err
	}

	p := pageViewData{
		Title:     title,
		GithubUrl: githubUrl,
		Versions:  versions,
		Menu:      menu,
		Content:   template.HTML(html),
	}
	if err := h.template.ExecuteTemplate(w, layoutFile, p); err != nil {
		return fmt.Errorf("could not render the layout: %v", err)
	}

	return nil
}

type pageViewData struct {
	Title     string
	GithubUrl string
	Versions  []versionOptionViewData
	Menu      []menuSectionViewData
	Content   any
}

type versionOptionViewData struct {
	Version  string
	Url      string
	IsActive bool
}

type menuSectionViewData struct {
	Title string
	Items []menuItemViewData
}

type menuItemViewData struct {
	Title    string
	Url      string
	IsActive bool
}

func (h *DocsHandler) githubUrl(file string) (string, error) {
	u, err := url.Parse(fmt.Sprintf("%s/tree/main/docs/", h.config.githubUrl))
	if err != nil {
		return "", fmt.Errorf("could not get parse Github url: %w", err)
	}
	return u.JoinPath(file).String(), nil
}

func (h *DocsHandler) versionsViewData(current *docsVersion, info *docsFile) ([]versionOptionViewData, error) {
	var options []versionOptionViewData
	for _, v := range h.versions {
		// If available, link to the same file.
		var u string
		if f, ok := v.srcLookup[info.srcPath]; ok {
			u = h.fileUrl(f).String()
		} else {
			// Default to the root page of the version.
			u = h.config.siteUrl.JoinPath(v.name).String()
		}

		options = append(options, versionOptionViewData{
			Version:  v.name,
			Url:      u,
			IsActive: v.name == current.name,
		})
	}
	return options, nil
}

func (h *DocsHandler) menuViewData(v *docsVersion, info *docsFile) ([]menuSectionViewData, error) {
	var sections []menuSectionViewData
	for _, item := range v.menu {
		if !item.IsDir {
			continue
		}
		s := menuSectionViewData{
			Title: item.Title,
		}
		for _, sub := range item.Items {
			if sub.IsDir {
				continue
			}
			f, ok := v.srcLookup[sub.Path]
			if !ok {
				panic("menu item url does not match docs file")
			}
			itm := menuItemViewData{
				Title:    sub.Title,
				Url:      h.fileUrl(f).String(),
				IsActive: sub.Path == info.srcPath,
			}
			s.Items = append(s.Items, itm)
		}
		sections = append(sections, s)
	}
	return sections, nil
}

func (h *DocsHandler) loadTemplates() error {
	t := template.New("")
	t.Funcs(map[string]any{
		"asset": func(file string) string {
			return h.config.siteUrl.JoinPath(file).String()
		},
	})
	err := fs.WalkDir(h.templateFs, templateDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error walking template directory: %w", err)
		}
		if entry.IsDir() {
			return nil
		}
		_, err = t.New(filepath.Base(path)).ParseFS(h.templateFs, path)
		if err != nil {
			return fmt.Errorf("could not parse template file %v: %w", path, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("could not parse templates: %w", err)
	}
	h.template = t
	return nil
}

func (h *DocsHandler) fileUrl(f *docsFile) *url.URL {
	u := h.config.siteUrl.JoinPath(f.version.name, f.dstPath)
	if dir, file := path.Split(u.Path); file == "index.html" {
		u.Path = dir
		return u
	}
	u.Path = strings.TrimSuffix(u.Path, ".html")
	return u
}

func (h *DocsHandler) rewriteContentUrl(v *docsVersion, content *docsFile, link string) (string, error) {
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
		srcPath = filepath.Join(filepath.Dir(content.srcPath), u.Path)
	}
	file, ok := v.srcLookup[srcPath]
	if !ok {
		// todo: log warning or cause error.
		return link, nil
	}

	ru := h.fileUrl(file)
	ru.ForceQuery = u.ForceQuery
	ru.RawQuery = u.RawQuery
	ru.Fragment = u.Fragment
	ru.RawFragment = u.RawFragment

	return ru.String(), nil
}
