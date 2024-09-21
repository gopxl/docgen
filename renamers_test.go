package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSectionDirectoryRenamer_Rename(t *testing.T) {
	r := SectionDirectoryRenamer{}

	// Untouched (except slugification)
	assert.Equal(t, "tutorial/Foo.md", r.Rename("tutorial/Foo.md"))
	assert.Equal(t, "1-tutorial/Foo.md", r.Rename("1 Tutorial/Foo.md"))
	assert.Equal(t, "tutorial/2. Foo.md", r.Rename("Tutorial/2. Foo.md"))
	assert.Equal(t, "tutorial/Foo.md", r.Rename(".Tutorial/Foo.md"))
	assert.Equal(t, "tutorial/01. Subdir/Foo.md", r.Rename("Tutorial/01. Subdir/Foo.md"))

	// Renamed
	assert.Equal(t, "tutorial/2. Foo.md", r.Rename("1. Tutorial/2. Foo.md"))
	assert.Equal(t, "tutorial/02. Foo.md", r.Rename("01. Tutorial/02. Foo.md"))
	assert.Equal(t, "first-steps-and-introduction/Foo.md", r.Rename("First Steps & introduction/Foo.md"))
	assert.Equal(t, "tutorial/images/foo.png", r.Rename("01. Tutorial/images/foo.png"))
}

func TestPageFileRenamer_Rename(t *testing.T) {
	r := PageFileRenamer{}

	// Untouched
	assert.Equal(t, "Tutorial/foo.md", r.Rename("Tutorial/foo.md"))
	assert.Equal(t, "01. Tutorial/foo.md", r.Rename("01. Tutorial/foo.md"))

	// Renamed
	assert.Equal(t, "Tutorial/foo.md", r.Rename("Tutorial/01. Foo.md"))
	assert.Equal(t, "Tutorial/foo-bar.md", r.Rename("Tutorial/01. Foo Bar.md"))
	assert.Equal(t, "Tutorial/1-foo.md", r.Rename("Tutorial/1 Foo.md"))
}
