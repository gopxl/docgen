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

func newBundle(toolingFs fs.FS, versions []Version, docsDir, repoUrl string, rootUrl *url.URL) (*Bundle, error) {
	b := NewBundler()

	b.FromFs(toolingFs).
		TakeDir("public").
		PutInDir(".")
	b.FromFs(toolingFs).
		TakeGlob("node_modules/prismjs/components", "*.min.js").
		PutInDir("vendor/prismjs/components")
	b.FromFs(toolingFs).
		TakeFile("node_modules/prismjs/plugins/autoloader/prism-autoloader.min.js").
		PutInDir("vendor/prismjs/plugins/autoloader")

	// Compile docs for each version.
	for _, v := range versions {
		docsFs, err := fs.Sub(v.FS, docsDir)
		if err != nil {
			return nil, fmt.Errorf("could not open the %s documentation subdirectory: %w", docsDir, err)
		}

		renderer := NewPageRenderer(
			toolingFs,
			"resources/views",
			"layout.gohtml",
			func() ([]MenuItem, error) {
				return NewMenuFromFs(docsFs)
			},
			versions,
			repoUrl,
		)

		b.FromFs(docsFs).
			TakeGlob(".", "**/*.md").
			CompileWith(NewMarkdownCompiler(renderer)).
			PutInDir(v.Name)

		b.FromFs(docsFs).
			TakeDir(".").
			Filter(func(file string) bool {
				return filepath.Ext(file) != ".md"
			}).
			PutInDir(v.Name)
	}

	return b.Compile(rootUrl)
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

	log.Printf("url: %s", rootUrl.String())
	log.Printf("repository url: %s", repoUrl)
	log.Printf("repository directory: %s", repoDir)
	log.Printf("docs directory: %s", docsDir)

	versions, err := GetDocVersions(repoDir, docsDir, mainBranch, dev)
	if err != nil {
		log.Fatalf("could not determine publishable versions: %w", err)
	}

	if debug {
		bun, err := newBundle(embeddedFs, versions, docsDir, repoUrl, rootUrl)
		if err != nil {
			log.Fatalf("could not create bundle: %v", err)
		}
		fmt.Println(bun.DestFiles())
	}

	if serve {
		log.Println("Starting development server...")

		// Override root url.
		rootUrl, err = url.Parse("http://localhost:8080")
		if err != nil {
			log.Fatalf("could not parse root url: %v", err)
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
			pth := path.Clean(strings.TrimLeft(request.URL.Path, "/"))
			if len(path.Ext(pth)) == 0 {
				pth = path.Join(pth, "index.html")
			}

			b, err := newBundle(embeddedFs, versions, docsDir, repoUrl, rootUrl)
			if err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				_, _ = writer.Write([]byte(fmt.Sprintf("could not create bundle: %v", err)))
				return
			}

			var buf bytes.Buffer
			err = b.CompileFileToWriter(pth, &buf)
			if errors.Is(err, fs.ErrNotExist) {
				writer.WriteHeader(http.StatusNotFound)
				_, _ = writer.Write([]byte("Not Found"))
				return
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
		b, err := newBundle(embeddedFs, versions, docsDir, repoUrl, rootUrl)
		if err != nil {
			log.Fatalf("could not create bundle: %v", err)
		}
		err = b.CompileAllToDir(destDir)
		if err != nil {
			log.Fatal(err)
		}
	}
}
