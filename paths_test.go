package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathRewriter_ModifyPath(t *testing.T) {
	r := PathRewriter{}

	assert.Equal(t, "tutorial/foo.html", r.ModifyPath("tutorial/Foo.md", false))
	assert.Equal(t, "tutorial/foo.html", r.ModifyPath("01. Tutorial/01. Foo.md", false))
	assert.Equal(t, "tutorial", r.ModifyPath("01. Tutorial", true))

	assert.Equal(t, "tutorial/foo.html", r.ModifyPath("tutorial/foo.html", false))
	assert.Equal(t, "tutorial/01. Images/01. foo.png", r.ModifyPath("01. Tutorial/01. Images/01. foo.png", false))
}
