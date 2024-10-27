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
	"strconv"
	"strings"

	"github.com/gopxl/docgen/internal/bundler"
	"github.com/joho/godotenv"
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

	err = godotenv.Load()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("error loading .env file: %v", err)
	}

	var serve bool
	var debug bool
	flag.BoolVar(&serve, "serve", false, "serve the site through a webserver for development")
	flag.BoolVar(&debug, "debug", false, "print debugging information")
	flag.Parse()

	siteUrlStr := os.Getenv("SITE_URL")
	githubUrl := os.Getenv("GITHUB_URL")
	repoPath := filepath.Clean(os.Getenv("REPOSITORY_PATH"))
	docsDir := filepath.Clean(os.Getenv("DOCS_DIR"))
	outputDir := os.Getenv("OUTPUT_DIR")
	mainBranch := os.Getenv("MAIN_BRANCH")
	withWorkingDirStr := os.Getenv("WORKING_DIRECTORY")
	withWorkingDir, err := strconv.ParseBool(withWorkingDirStr)
	if err != nil {
		withWorkingDir = false
	}

	siteUrl, err := url.Parse(siteUrlStr)
	if err != nil {
		log.Fatalf("could not parse root url %s: %v", siteUrlStr, err)
	}

	config := &Config{
		siteUrl:        siteUrl,
		githubUrl:      githubUrl,
		repositoryPath: repoPath,
		docsDir:        docsDir,
		outputDir:      outputDir,
		mainBranch:     mainBranch,
		withWorkingDir: withWorkingDir,
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
		siteUrl, err = url.Parse("http://localhost:8080")
		if err != nil {
			log.Fatalf("could not parse root url: %v", err)
		}
		devConfig := *&config // shallow copy
		devConfig.siteUrl = siteUrl

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
			b, err := newBundle(embeddedFs, devConfig)
			if err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				_, _ = writer.Write([]byte(fmt.Sprintf("could not create bundle: %v", err)))
				return
			}

			pth := path.Clean(strings.TrimLeft(request.URL.Path, "/"))
			aliases := []string{
				pth,
				pth + ".html",
				path.Join(pth, "index.html"),
			}
			var buf bytes.Buffer
			var found bool
			for _, pth := range aliases {
				err = b.WriteFileTo(pth, &buf)
				if errors.Is(err, fs.ErrNotExist) {
					// Try an alias.
					continue
				}
				if err != nil {
					writer.WriteHeader(http.StatusInternalServerError)
					_, _ = writer.Write([]byte(fmt.Sprintf("could not write file: %v", err)))
					return
				}
				found = true
			}
			if !found {
				writer.WriteHeader(http.StatusNotFound)
				_, _ = writer.Write([]byte("Not Found"))
				return
			}

			writer.Header().Add("Content-Type", mime.TypeByExtension(filepath.Ext(pth)))
			_, _ = writer.Write(buf.Bytes())
		})
		s := &http.Server{
			Addr:    fmt.Sprintf(":%s", siteUrl.Port()),
			Handler: mux,
		}
		log.Printf("listening on %v", siteUrl.String())
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
		err = b.StoreInDir(config.outputDir)
		if err != nil {
			log.Fatal(err)
		}
	}
}
