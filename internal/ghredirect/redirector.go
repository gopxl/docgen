package ghredirect

import (
	"bytes"
	"fmt"
	"html/template"
	"io"

	"github.com/gopxl/docgen/internal/bundler"
)

type Redirector struct {
	b    *bundler.Bundler
	tmpl *template.Template
}

func NewRedirector(b *bundler.Bundler, tmpl string) (*Redirector, error) {
	t, err := template.New("redirect.gohtml").Parse(tmpl)
	if err != nil {
		return nil, fmt.Errorf("could not parse redirect template: %w", err)
	}

	return &Redirector{
		b:    b,
		tmpl: t,
	}, nil
}

func (r *Redirector) RedirectToTaggedFile(from, tag, to string) {
	r.b.Add(
		&bundler.EmptyFileSource{Path: "index.html"},
		bundler.Pipeline(
			&redirectModifier{r: r, tag: tag, dst: to},
		),
		bundler.StoreIn(from),
	)
}

type redirectModifier struct {
	r   *Redirector
	tag string
	dst string
}

func (rm *redirectModifier) ModifyContent(r io.Reader, w io.Writer, ctx *bundler.Context) error {
	dst, err := ctx.TaggedFileUrl(rm.tag, rm.dst)
	if err != nil {
		// todo: deal with fs.ErrNotExist errors better.
		return fmt.Errorf("could not get destination url for redirect: %w", err)
	}

	viewData := struct {
		RedirectUrl string
	}{
		RedirectUrl: dst,
	}

	var buf bytes.Buffer
	err = rm.r.tmpl.Execute(&buf, viewData)
	if err != nil {
		return fmt.Errorf("error executing redirect template: %w", err)
	}
	_, err = buf.WriteTo(w)
	return err
}
