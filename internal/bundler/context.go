package bundler

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

type Context struct {
	Bundle  *Bundle
	Mapping *Mapping
}

func (c *Context) GetUriSegment(i int) string {
	p := strings.Split(c.Mapping.storePath, "/")
	if i >= len(p) {
		return ""
	}
	return p[i]
}

func (c *Context) ToAbsUrl(file string) *url.URL {
	return c.Bundle.rootUrl.JoinPath(file)
}

func (c *Context) TaggedFileUrl(tag, file string) (string, error) {
	file = path.Clean(strings.TrimLeft(file, "/"))
	f, ok := c.Bundle.tagged[tag][file]
	if !ok {
		return "", fs.ErrNotExist
	}
	return c.ToAbsUrl(f.storePath).String(), nil
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
		//srcPath = filepath.Clean(strings.TrimLeft(u.Path, "/"))
		srcPath = u.Path
	} else {
		// relative to current file
		srcPath = filepath.Join(filepath.Dir(c.Mapping.SrcPath), u.Path)
	}
	if c.Mapping.tag == "" {
		return "", errors.New(fmt.Sprintf("cannot rewrite url relative to untagged file %s", c.Mapping.SrcPath))
	}
	//f, ok := c.Bundle.tagged[c.Mapping.tag][srcPath]
	//if !ok {
	//	return "", fs.ErrNotExist
	//}
	//return c.ToAbsUrl(f.storePath).String(), nil
	return c.TaggedFileUrl(c.Mapping.tag, srcPath)
}
