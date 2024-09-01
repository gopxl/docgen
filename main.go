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

func newBundle(toolingFs fs.FS, repository Repository, docsDir, repoUrl string, rootUrl *url.URL) (*Bundle, error) {
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

	// Check which versions have a docs directory.
	versions, err := repository.Versions()
	if err != nil {
		return nil, fmt.Errorf("could not get versions from repository: %w", err)
	}
	for i := 0; i < len(versions); i++ {
		v := versions[i]
		versionFs, err := repository.FS(v)
		if err != nil {
			return nil, fmt.Errorf("could not open the repository as a filesystem: %w", err)
		}
		_, err = versionFs.Open(docsDir)
		if errors.Is(err, fs.ErrNotExist) {
			versions = append(versions[:i], versions[i+1:]...)
			i--
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("could not open the %s documentation subdirectory: %w", docsDir, err)
		}
	}

	// Compile docs for each version.
	for _, v := range versions {
		versionFs, err := repository.FS(v)
		if err != nil {
			return nil, fmt.Errorf("could not open the repository as a filesystem: %w", err)
		}
		docsFs, err := fs.Sub(versionFs, docsDir)
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
			PutInDir(v.DisplayName())

		b.FromFs(docsFs).
			TakeDir(".").
			Filter(func(file string) bool {
				return filepath.Ext(file) != ".md"
			}).
			PutInDir(v.DisplayName())
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
	var docsDir string
	var destDir string
	var serve bool
	var debug bool

	flag.StringVar(&rootUrlStr, "url", "", "root url the files will be hosted under (https://owner.github.com/project)")
	flag.StringVar(&repoDir, "repository", ".", "path to the git repository")
	flag.StringVar(&repoUrl, "repository-url", ".", "GitHub url of the git repository (https://github.com/owner/project)")
	flag.StringVar(&docsDir, "docs", "docs", "the directory containing the documentation within the repository")
	flag.StringVar(&destDir, "dest", "generated", "path to the output directory")
	flag.BoolVar(&serve, "serve", false, "serve the site live through a webserver for development")
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

	repository, err := NewGitRepository(repoDir)
	if err != nil {
		log.Fatalf("could not open Git repository: %v", err)
	}

	if debug {
		bun, err := newBundle(embeddedFs, repository, docsDir, repoUrl, rootUrl)
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

			b, err := newBundle(embeddedFs, repository, docsDir, repoUrl, rootUrl)
			if err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				writer.Write([]byte(fmt.Sprintf("could not create bundle: %v", err)))
				return
			}

			var buf bytes.Buffer
			err = b.CompileFileToWriter(pth, &buf)
			if errors.Is(err, fs.ErrNotExist) {
				writer.WriteHeader(http.StatusNotFound)
				writer.Write([]byte("Not Found"))
				return
			}
			if err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				writer.Write([]byte(fmt.Sprintf("could not write file: %v", err)))
				return
			}

			writer.Header().Add("Content-Type", mime.TypeByExtension(filepath.Ext(pth)))
			writer.Write(buf.Bytes())
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
		b, err := newBundle(embeddedFs, repository, docsDir, repoUrl, rootUrl)
		if err != nil {
			log.Fatalf("could not create bundle: %v", err)
		}
		err = b.CompileAllToDir(destDir)
		if err != nil {
			log.Fatal(err)
		}
	}
}
