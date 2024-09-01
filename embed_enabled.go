//go:build embed

package main

import "embed"

//go:embed resources/views/*
//go:embed public/*
//go:embed node_modules/prismjs/components/*.min.js
//go:embed node_modules/prismjs/plugins/autoloader/prism-autoloader.min.js
var embeddedFs embed.FS
