package main

import (
	"bytes"
	"fmt"
	"net/url"
)

type Config struct {
	rootUrl        *url.URL // URL the website is served under. Used for determining absolute paths of resources and links.
	repositoryDir  string   // filesystem path to the Git repository
	docsDir        string   // documentation directory relative to the repository root
	mainBranch     string   // name of the main branch
	githubUrl      string   // GitHub repository url
	withWorkingDir bool     // whether to include the current working directory as a published version
}

func (c *Config) String() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Root URL:                %s\n", c.rootUrl.String()))
	buf.WriteString(fmt.Sprintf("Repository directory:    %s\n", c.repositoryDir))
	buf.WriteString(fmt.Sprintf("Documentation directory: %s\n", c.docsDir))
	buf.WriteString(fmt.Sprintf("Main branch:             %s\n", c.mainBranch))
	buf.WriteString(fmt.Sprintf("GitHub URL:              %s\n", c.githubUrl))
	buf.WriteString(fmt.Sprintf("With working directory:  %t\n", c.withWorkingDir))
	return buf.String()
}
