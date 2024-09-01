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
	repoUrl     string
}

type MenuFunc func() ([]MenuItem, error)

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

func NewPageRenderer(templateFs fs.FS, templateDir, layoutFile string, menuFunc MenuFunc, versions []Version, repoUrl string) *PageRenderer {
	return &PageRenderer{
		templateFs:  templateFs,
		templateDir: templateDir,
		layoutFile:  layoutFile,
		menuFunc:    menuFunc,
		versions:    versions,
		repoUrl:     repoUrl,
	}
}

func (pr *PageRenderer) Render(w io.Writer, c *Context, content any) error {
	funcMap := map[string]any{
		"asset": func(file string) string {
			return c.ToAbsUrl(file).String()
		},
	}

	t, err := pr.loadTemplate(funcMap)
	if err != nil {
		return err
	}

	vm, err := pr.pageViewData(c, content)
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

func (pr *PageRenderer) pageViewData(c *Context, content any) (page, error) {
	githubRoot, err := url.Parse(fmt.Sprintf("%s/tree/main/docs/", pr.repoUrl))
	if err != nil {
		return page{}, fmt.Errorf("could not get parse Github url: %w", err)
	}

	versions, err := pr.versionsViewData(c)
	if err != nil {
		return page{}, err
	}

	menu, err := pr.menuViewData(c)
	if err != nil {
		return page{}, err
	}

	return page{
		Title:     strings.TrimSuffix(filepath.Base(c.mapping.dstPath), filepath.Ext(c.mapping.dstPath)), // todo: add website title.
		GithubUrl: githubRoot.JoinPath(c.mapping.srcPath).String(),
		Versions:  versions,
		Menu:      menu,
		Content:   content,
	}, nil
}

func (pr *PageRenderer) versionsViewData(c *Context) ([]pageVersionOption, error) {
	var vs []pageVersionOption
	for _, v := range pr.versions {
		// todo: if the current page exists in the target version,
		//       make the url point to that page.

		vs = append(vs, pageVersionOption{
			Version:  v.Name,
			Url:      c.ToAbsUrl(v.Name).String(),
			IsActive: c.GetUriSegment(0) == v.Name,
		})
	}
	return vs, nil
}

func (pr *PageRenderer) menuViewData(c *Context) ([]pageMenuSection, error) {
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
			uri := path.Join(c.GetUriSegment(0), sub.Path)
			itm := pageMenuItem{
				Title:    sub.Title,
				Url:      c.ToAbsUrl(uri).String(),
				IsActive: c.mapping.dstPath == uri,
			}
			s.Items = append(s.Items, itm)
		}
		sections = append(sections, s)
	}
	return sections, nil
}
