package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
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

func newBundle(toolingFs fs.FS, versions []Version, docsDir, repoUrl string, rootUrl *url.URL) (*bundler.Bundle, error) {
	b := bundler.NewBundler()

	rtf, err := embeddedFs.Open("resources/views/redirect.gohtml")
	if err != nil {
		return nil, err
	}
	rt, err := io.ReadAll(rtf)
	if err != nil {
		return nil, err
	}
	b.SetRedirectTemplate(string(rt))

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

		s, err := readSettings(docsFs)
		if err != nil {
			return nil, err
		}

		pageRenamer := bundler.NewCompositeRewriter(
			&SectionDirectoryRenamer{},
			&PageFileRenamer{},
			&MarkdownCompiler{},
		)

		menuItems, err := NewMenuFromFs(docsFs)

		// Add redirect from root url to default version.
		if v.IsDefault {
			b.Redirect(&url.URL{Path: "/index.html"}, &url.URL{Path: v.Name}).
				PutInDir(".")
		}

		// Default redirect: from version root to first section.
		if len(menuItems) > 0 {
			b.Redirect(&url.URL{Path: "/index.html"}, &url.URL{Path: menuItems[0].Items[0].Path}).
				WithTargetFs(docsFs).
				PutInDir(v.Name)
		}

		// Default redirect: from section root to first page in section.
		for _, item := range menuItems {
			b.Redirect(&url.URL{Path: (&SectionDirectoryRenamer{}).Rename(item.Path + "/index.html")}, &url.URL{Path: item.Items[0].Path}).
				WithTargetFs(docsFs).
				PutInDir(v.Name)
		}

		// Add configured redirects.
		for from, to := range s.Redirects {
			b.Redirect(&from, &to).
				WithTargetFs(docsFs).
				PutInDir(v.Name)
		}

		renderer := NewPageRenderer(
			toolingFs,
			"resources/views",
			"layout.gohtml",
			menuItems,
			versions,
			repoUrl,
		)

		b.FromFs(docsFs).
			TakeGlob(".", "**/*.md").
			RenameWith(pageRenamer).
			CompileWith(NewMarkdownCompiler(renderer)).
			PutInDir(v.Name)

		b.FromFs(docsFs).
			TakeDir(".").
			Filter(func(file string) bool {
				return filepath.Ext(file) != ".md" && file != settingsFile
			}).
			RenameWith(&SectionDirectoryRenamer{}).
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
		log.Fatalf("could not determine publishable versions: %v", err)
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
			b, err := newBundle(embeddedFs, versions, docsDir, repoUrl, rootUrl)
			if err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				_, _ = writer.Write([]byte(fmt.Sprintf("could not create bundle: %v", err)))
				return
			}

			pth := path.Clean(strings.TrimLeft(request.URL.Path, "/"))
			var buf bytes.Buffer
			err = b.CompileFileToWriter(pth, &buf)
			if errors.Is(err, fs.ErrNotExist) {
				// Try index.html instead.
				pth = path.Join(pth, "index.html")
				err = b.CompileFileToWriter(pth, &buf)
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
