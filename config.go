package main

import (
	"bytes"
	"fmt"
	"net/url"
)

type Config struct {
	siteUrl        *url.URL // URL the website is served under. Used for determining absolute paths of resources and links.
	githubUrl      string   // GitHub repository url
	repositoryPath string   // filesystem path to the Git repository
	docsDir        string   // documentation directory relative to the repository root
	outputDir      string   // output directory of the static site generation process
	mainBranch     string   // name of the main branch
	withWorkingDir bool     // whether to include the current working directory as a published version
}

func (c *Config) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Site URL:                %s\n", c.siteUrl.String()))
	buf.WriteString(fmt.Sprintf("Repository path:         %s\n", c.repositoryPath))
	buf.WriteString(fmt.Sprintf("Documentation directory: %s\n", c.docsDir))
	buf.WriteString(fmt.Sprintf("Output directory:        %s\n", c.outputDir))
	buf.WriteString(fmt.Sprintf("Main branch:             %s\n", c.mainBranch))
	buf.WriteString(fmt.Sprintf("GitHub URL:              %s\n", c.githubUrl))
	buf.WriteString(fmt.Sprintf("With working directory:  %t\n", c.withWorkingDir))
	return buf.String()
}
