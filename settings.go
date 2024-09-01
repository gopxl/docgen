package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"

	"github.com/goccy/go-yaml"
)

const settingsFile = "docgen.yml"

type Settings struct {
	Redirects map[url.URL]url.URL
}

func readSettings(filesys fs.FS) (*Settings, error) {
	f, err := filesys.Open(settingsFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Settings{}, nil
		}
		return nil, fmt.Errorf("could not open %s: %w", settingsFile, err)
	}
	yml, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", settingsFile, err)
	}
	s := &Settings{}
	if err := yaml.UnmarshalWithOptions(yml, s, yaml.CustomUnmarshaler(unmarshalYamlUrl)); err != nil {
		return nil, fmt.Errorf("error decoding %s: %w", settingsFile, err)
	}
	return s, nil
}

func unmarshalYamlUrl(u *url.URL, bytes []byte) error {
	var ustr string
	err := yaml.Unmarshal(bytes, &ustr)
	if err != nil {
		return fmt.Errorf("could not unmarshall url string: %w", err)
	}

	parsed, err := url.Parse(ustr)
	if err != nil {
		return err
	}
	*u = *parsed
	return nil
}
