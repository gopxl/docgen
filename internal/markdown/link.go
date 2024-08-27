package markdown

import (
	"net/url"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type AbsoluteLinkTargetBlankTransformer struct {
}

// NewAbsoluteLinkTargetBlankTransformer transforms links with absolute urls so
// that they open in a new tab.
func NewAbsoluteLinkTargetBlankTransformer() *AbsoluteLinkTargetBlankTransformer {
	return &AbsoluteLinkTargetBlankTransformer{}
}

func (t *AbsoluteLinkTargetBlankTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if link, ok := n.(*ast.Link); ok {
			if t.shouldOpenInNewTab(string(link.Destination)) {
				link.SetAttributeString("target", "_blank")
			}
			return ast.WalkContinue, nil
		}
		// todo: autolinks
		return ast.WalkContinue, nil
	})
}

func (t *AbsoluteLinkTargetBlankTransformer) shouldOpenInNewTab(link string) bool {
	u, err := url.Parse(link)
	if err != nil {
		// Ignore
		return true
	}
	return u.IsAbs()
}
