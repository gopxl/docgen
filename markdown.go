package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"strings"

	"github.com/MarkKremer/gopxl-docs/internal/markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"
)

type Renderer interface {
	Render(w io.Writer, request *Request, content any) error
}

type MarkdownCompiler struct {
	layout Renderer
}

func NewMarkdownCompiler(layout Renderer) *MarkdownCompiler {
	return &MarkdownCompiler{
		layout: layout,
	}
}

func (m *MarkdownCompiler) OutputFileName(oldName string) (newName string) {
	return strings.TrimSuffix(oldName, filepath.Ext(oldName)) + ".html"
}

func (m *MarkdownCompiler) Compile(dst io.Writer, src io.Reader, request *Request) error {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(
				util.Prioritized(markdown.NewAbsoluteLinkTargetBlankTransformer(), 1),
				util.Prioritized(markdown.NewUrlTransformer(func(url string) string {
					rewritten, err := request.bundler.RewriteContentUrl(request, url)
					if err != nil {
						return url
					}
					return rewritten
				}), 1),
			),
		),
	)

	buf, err := io.ReadAll(src)
	if err != nil {
		return fmt.Errorf("could not read from source: %w", err)
	}
	var content bytes.Buffer
	if err := md.Convert(buf, &content); err != nil {
		return fmt.Errorf("could not convert Markdown: %w", err)
	}

	return m.layout.Render(dst, request, template.HTML(content.String()))
}
