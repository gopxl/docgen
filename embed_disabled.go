//go:build !embed

package main

import (
	"os"
)

// When no embed build tag is specified, the local filesystem is used instead.
var embeddedFs = os.DirFS(".")
