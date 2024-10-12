package bundler

import (
	"fmt"
	"io"
)

type PathModifier interface {
	ModifyPath(p string) string
}

type ContentModifier interface {
	ModifyContent(r io.Reader, w io.Writer, ctx *Context) error
}

type ModifierSlice []interface{}

func (ms ModifierSlice) ModifyPath(p string) string {
	for _, m := range ms {
		if m, ok := m.(PathModifier); ok {
			p = m.ModifyPath(p)
		}
	}
	return p
}

func (ms ModifierSlice) ModifyContent(r io.Reader, w io.Writer, ctx *Context) error {
	currentReader := r
	for i, m := range ms {
		if m, ok := m.(ContentModifier); ok {
			// Create an io.Pipe to connect the current modifier's output to the next one's input.
			pr, pw := io.Pipe()

			// Modify data from the current reader to the pipe writer.
			go func(m ContentModifier, r io.Reader, w *io.PipeWriter, ctx *Context, i int) {
				defer w.Close()
				if err := m.ModifyContent(r, w, ctx); err != nil {
					w.CloseWithError(fmt.Errorf("modifier %d returned error: %w", i, err))
				}
			}(m, currentReader, pw, ctx, i)

			// Set the next reader to be the pipe reader, which will feed the next modifier.
			currentReader = pr
		}
	}

	// The last modifier's output should go to the final output writer.
	_, err := io.Copy(w, currentReader)
	if err != nil {
		return fmt.Errorf("error writing final output: %w", err)
	}

	return nil
}
