package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/gopxl/docgen/internal/bundler"
	"github.com/gopxl/docgen/internal/markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"
)

type MarkdownRenderer struct {
}

func (m *MarkdownRenderer) ModifyPath(p string) string {
	return strings.TrimSuffix(p, filepath.Ext(p)) + ".html"
}

func (m *MarkdownRenderer) ModifyContent(r io.Reader, w io.Writer, ctx *bundler.Context) error {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(
				util.Prioritized(markdown.NewAbsoluteLinkTargetBlankTransformer(), 1),
				util.Prioritized(markdown.NewUrlTransformer(func(url string) string {
					rewritten, err := ctx.RewriteContentUrl(url)
					if err != nil {
						// Ignore error and return original url.
						return url
					}
					return rewritten
				}), 1),
			),
		),
	)

	buf, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("could not read from source: %w", err)
	}
	if err := md.Convert(buf, w); err != nil {
		return fmt.Errorf("could not convert Markdown: %w", err)
	}
	return nil
}
