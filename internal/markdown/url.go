package markdown

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type UrlTransformer struct {
	transform func(string) string
}

// NewUrlTransformer transforms urls from links and images using the provided
// transform function.
func NewUrlTransformer(transform func(string) string) *UrlTransformer {
	return &UrlTransformer{transform: transform}
}

func (t *UrlTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if img, ok := n.(*ast.Image); ok {
			img.Destination = []byte(t.transform(string(img.Destination)))
			return ast.WalkContinue, nil
		}
		if link, ok := n.(*ast.Link); ok {
			link.Destination = []byte(t.transform(string(link.Destination)))
			return ast.WalkContinue, nil
		}
		// todo: autolinks
		return ast.WalkContinue, nil
	})
}
