package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gopxl/docgen/internal/bundler"
)

func init() {
	err := mime.AddExtensionType(".css", "text/css")
	if err != nil {
		panic(fmt.Errorf("could not register mime type: %w", err))
	}
	err = mime.AddExtensionType(".svg", "image/svg+xml")
	if err != nil {
		panic(fmt.Errorf("could not register mime type: %w", err))
	}
}

func newBundle(toolingFs fs.FS, config *Config) (*bundler.Bundle, error) {
	b := bundler.NewBundler()

	b.Add(
		bundler.NewFsDirHandler(
			toolingFs,
			"public",
			".",
		),
	)
	b.Add(
		bundler.NewFsGlobHandler(
			toolingFs,
			"node_modules/prismjs/components",
			"*.min.js",
			"vendor/prismjs/components",
		),
	)
	b.Add(
		bundler.NewFsFileHandler(
			toolingFs,
			"node_modules/prismjs/plugins/autoloader/prism-autoloader.min.js",
			"vendor/prismjs/plugins/autoloader/prism-autoloader.min.js",
		),
	)

	docsHandler, err := NewDocsHandler(toolingFs, config)
	if err != nil {
		return nil, err
	}
	b.Add(docsHandler)

	return b.Compile()
}

func main() {
	workingDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("could not get the current working directory: %v", err)
	}
	log.Printf("current working directory: %s", workingDir)

	var rootUrlStr string
	var repoDir string
	var repoUrl string
	var mainBranch string
	var docsDir string
	var destDir string
	var serve bool
	var dev bool
	var debug bool

	flag.StringVar(&rootUrlStr, "url", "", "root url the files will be hosted under (https://owner.github.com/project)")
	flag.StringVar(&repoDir, "repository", ".", "path to the git repository")
	flag.StringVar(&repoUrl, "repository-url", ".", "GitHub url of the git repository (https://github.com/owner/project)")
	flag.StringVar(&mainBranch, "main-branch", "main", "Branch to include in the version list")
	flag.StringVar(&docsDir, "docs", "docs", "the directory containing the documentation within the repository")
	flag.StringVar(&destDir, "dest", "generated", "path to the output directory")
	flag.BoolVar(&serve, "serve", false, "serve the site through a webserver for development")
	flag.BoolVar(&dev, "dev", false, "include the local working directory of the repository as a published version")
	flag.BoolVar(&debug, "debug", false, "print debugging information")
	flag.Parse()

	repoDir = filepath.Clean(repoDir)
	docsDir = filepath.Clean(docsDir)

	rootUrl, err := url.Parse(rootUrlStr)
	if err != nil {
		log.Fatalf("could not parse root url %s: %v", rootUrlStr, err)
	}

	config := &Config{
		rootUrl:        rootUrl,
		repositoryDir:  repoDir,
		docsDir:        docsDir,
		mainBranch:     mainBranch,
		githubUrl:      repoUrl,
		withWorkingDir: dev,
	}
	log.Printf("config:\n%v", config)

	if debug {
		bun, err := newBundle(embeddedFs, config)
		if err != nil {
			log.Fatalf("could not create bundle: %v", err)
		}
		fmt.Println(bun.Files())
	}

	if serve {
		log.Println("Starting development server...")

		// Override root url.
		rootUrl, err = url.Parse("http://localhost:8080")
		if err != nil {
			log.Fatalf("could not parse root url: %v", err)
		}
		devConfig := *&config // shallow copy
		devConfig.rootUrl = rootUrl

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
			b, err := newBundle(embeddedFs, devConfig)
			if err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				_, _ = writer.Write([]byte(fmt.Sprintf("could not create bundle: %v", err)))
				return
			}

			pth := path.Clean(strings.TrimLeft(request.URL.Path, "/"))
			var buf bytes.Buffer
			err = b.WriteFileTo(pth, &buf)
			if errors.Is(err, fs.ErrNotExist) {
				// Try index.html instead.
				pth = path.Join(pth, "index.html")
				err = b.WriteFileTo(pth, &buf)
				if errors.Is(err, fs.ErrNotExist) {
					writer.WriteHeader(http.StatusNotFound)
					_, _ = writer.Write([]byte("Not Found"))
					return
				}
			}
			if err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				_, _ = writer.Write([]byte(fmt.Sprintf("could not write file: %v", err)))
				return
			}

			writer.Header().Add("Content-Type", mime.TypeByExtension(filepath.Ext(pth)))
			_, _ = writer.Write(buf.Bytes())
		})
		s := &http.Server{
			Addr:    fmt.Sprintf(":%s", rootUrl.Port()),
			Handler: mux,
		}
		log.Printf("listening on %v", rootUrl.String())
		err = s.ListenAndServe()
		if err != nil {
			log.Fatalf("could not serve development server: %v", err)
		}
	} else {
		log.Println("compiling...")
		b, err := newBundle(embeddedFs, config)
		if err != nil {
			log.Fatalf("could not create bundle: %v", err)
		}
		err = b.StoreInDir(destDir)
		if err != nil {
			log.Fatal(err)
		}
	}
}
