package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/gopxl/docgen/internal/bundler"
)

type PageRenderer struct {
	templateFs  fs.FS
	templateDir string
	layoutFile  string
	menuItems   []MenuItem
	versions    []Version
	repoUrl     string
}

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

func NewPageRenderer(templateFs fs.FS, templateDir, layoutFile string, menuItems []MenuItem, versions []Version, repoUrl string) *PageRenderer {
	return &PageRenderer{
		templateFs:  templateFs,
		templateDir: templateDir,
		layoutFile:  layoutFile,
		menuItems:   menuItems,
		versions:    versions,
		repoUrl:     repoUrl,
	}
}

func (pr *PageRenderer) ModifyContent(r io.Reader, w io.Writer, ctx *bundler.Context) error {
	funcMap := map[string]any{
		"asset": func(file string) string {
			return ctx.ToAbsUrl(file).String()
		},
	}

	var content bytes.Buffer
	_, err := content.ReadFrom(r)
	if err != nil {
		return fmt.Errorf("error reading from source reader: %w", err)
	}

	t, err := pr.loadTemplate(funcMap)
	if err != nil {
		return err
	}

	vm, err := pr.pageViewData(ctx, template.HTML(content.String()))
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

func (pr *PageRenderer) pageViewData(c *bundler.Context, content any) (page, error) {
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
		Title:     stripNumberPrefix(strings.TrimSuffix(filepath.Base(c.Mapping.SrcPath), filepath.Ext(c.Mapping.SrcPath))),
		GithubUrl: githubRoot.JoinPath(c.Mapping.SrcPath).String(),
		Versions:  versions,
		Menu:      menu,
		Content:   content,
	}, nil
}

func (pr *PageRenderer) versionsViewData(c *bundler.Context) ([]pageVersionOption, error) {
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

func (pr *PageRenderer) menuViewData(c *bundler.Context) ([]pageMenuSection, error) {
	var sections []pageMenuSection
	for _, item := range pr.menuItems {
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
			uri, err := c.RewriteContentUrl(path.Join("/", sub.Path))
			if err != nil {
				return nil, fmt.Errorf("could not get rewritten url to %s: %w", sub.Path, err)
			}
			itm := pageMenuItem{
				Title:    sub.Title,
				Url:      uri,
				IsActive: c.Mapping.SrcPath == sub.Path,
			}
			s.Items = append(s.Items, itm)
		}
		sections = append(sections, s)
	}
	return sections, nil
}
