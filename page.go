package main

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

type PageRenderer struct {
	templateFs  fs.FS
	templateDir string
	layoutFile  string
	menuFunc    MenuFunc
	versions    []Version
}

type MenuFunc func() ([]MenuItem, error)
type VersionsFunc func() ([]string, error)

type page struct {
	Title     string
	GithubUrl string
	Versions  []pageVersionOption
	Menu      []pageMenuSection
	Content   any
}

type pageVersionOption struct {
	Version  string
	Url      string
	IsActive bool
}

type pageMenuSection struct {
	Title string
	Items []pageMenuItem
}

type pageMenuItem struct {
	Title    string
	Url      string
	IsActive bool
}

func NewPageRenderer(templateFs fs.FS, templateDir, layoutFile string, menuFunc MenuFunc, versions []Version) *PageRenderer {
	return &PageRenderer{
		templateFs:  templateFs,
		templateDir: templateDir,
		layoutFile:  layoutFile,
		menuFunc:    menuFunc,
		versions:    versions,
	}
}

func (pr *PageRenderer) Render(w io.Writer, request *Request, content any) error {
	funcMap := map[string]any{
		"asset": func(file string) string {
			return request.UrlTo(file).String()
		},
	}

	t, err := pr.loadTemplate(funcMap)
	if err != nil {
		return err
	}

	vm, err := pr.pageViewData(request, content)
	if err != nil {
		return err
	}

	if err := t.ExecuteTemplate(w, pr.layoutFile, vm); err != nil {
		return fmt.Errorf("could not render the layout: %v", err)
	}

	return nil
}

func (pr *PageRenderer) loadTemplate(funcMap template.FuncMap) (*template.Template, error) {
	t := template.New("")
	t.Funcs(funcMap)
	err := fs.WalkDir(pr.templateFs, pr.templateDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error walking template directory: %w", err)
		}
		if d.IsDir() {
			return nil
		}
		_, err = t.New(filepath.Base(path)).ParseFS(pr.templateFs, path)
		if err != nil {
			return fmt.Errorf("could not parse template file %v: %w", path, err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not parse templates: %w", err)
	}
	return t, nil
}

func (pr *PageRenderer) pageViewData(request *Request, content any) (page, error) {
	githubRoot, err := url.Parse("https://github.com/gopxl/pixel/tree/main/docs/") // todo: don't hardcode url
	if err != nil {
		return page{}, fmt.Errorf("could not get parse Github url: %w", err)
	}

	versions, err := pr.versionsViewData(request)
	if err != nil {
		return page{}, err
	}

	menu, err := pr.menuViewData(request)
	if err != nil {
		return page{}, err
	}

	return page{
		Title:     strings.TrimSuffix(filepath.Base(request.uri), filepath.Ext(request.uri)), // todo: add website title.
		GithubUrl: githubRoot.JoinPath(request.srcPath).String(),
		Versions:  versions,
		Menu:      menu,
		Content:   content,
	}, nil
}

func (pr *PageRenderer) versionsViewData(request *Request) ([]pageVersionOption, error) {
	versions := pr.versions
	// Reverse order.
	for i, j := 0, len(versions)-1; i < j; i, j = i+1, j-1 {
		versions[i], versions[j] = versions[j], versions[i]
	}

	var vs []pageVersionOption
	for _, v := range versions {
		// todo: if the current page exists in the target version,
		//       make the url point to that page.

		vs = append(vs, pageVersionOption{
			Version:  v.DisplayName(),
			Url:      request.UrlTo(v.DisplayName()).String(),
			IsActive: request.bundleDstDir == v.DisplayName(),
		})
	}
	return vs, nil
}

func (pr *PageRenderer) menuViewData(request *Request) ([]pageMenuSection, error) {
	menu, err := pr.menuFunc()
	if err != nil {
		return nil, fmt.Errorf("could not get menu: %w", err)
	}

	var sections []pageMenuSection
	for _, item := range menu {
		if !item.IsDir {
			continue
		}
		s := pageMenuSection{
			Title: item.Title,
		}
		for _, sub := range item.Items {
			if sub.IsDir {
				continue
			}
			uri := path.Join(request.bundleDstDir, sub.Path) // todo: don't hardcode
			itm := pageMenuItem{
				Title:    sub.Title,
				Url:      request.UrlTo(uri).String(),
				IsActive: request.uri == uri,
			}
			s.Items = append(s.Items, itm)
		}
		sections = append(sections, s)
	}
	return sections, nil
}