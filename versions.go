package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"sort"

	"github.com/Masterminds/semver/v3"
)

type Version struct {
	Name      string
	Version   *semver.Version
	IsDefault bool
	FS        fs.FS
}

type preferredVersion byte

const (
	preferLatestTag preferredVersion = iota
	preferWorkingDir
	preferMainBranch
)

func GetDocVersions(config *Config) ([]Version, error) {
	var prefVersion preferredVersion
	if config.withWorkingDir {
		prefVersion = preferWorkingDir
	} else {
		prefVersion = preferLatestTag
	}

	repo, err := NewGitRepository(config.repositoryDir)
	if err != nil {
		return nil, fmt.Errorf("could not open git repository: %w", err)
	}
	tags, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("could not get tags from repository: %w", err)
	}

	var versions []Version
	latest := make(map[uint64]Version)
	for _, tag := range tags {
		v, err := semver.NewVersion(tag.Name())
		if err != nil {
			log.Printf("skipping tag %s: not a valid semver version", tag.Name())
			continue
		}
		filesys, err := repo.FS(tag)
		if err != nil {
			return nil, fmt.Errorf("could not open repository filesystem for tag %s: %w", tag.Name(), err)
		}
		_, err = filesys.Open(config.docsDir)
		if errors.Is(err, fs.ErrNotExist) {
			log.Printf("skipping tag %s: directory %s does not exist", tag.Name(), config.docsDir)
			continue
		}
		if other, ok := latest[v.Major()]; ok && v.LessThan(other.Version) {
			// Not the newest version.
			continue
		}
		versions = append(versions, Version{
			Name:    fmt.Sprintf("%d.x", v.Major()),
			Version: v,
			FS:      filesys,
		})
	}
	sort.Slice(versions, func(i, j int) bool {
		return versions[j].Version.LessThan(versions[i].Version)
	})
	if prefVersion == preferLatestTag {
		if len(versions) > 0 {
			versions[0].IsDefault = true
		} else {
			prefVersion = preferMainBranch
		}
	}

	branch, err := repo.Branch(config.mainBranch)
	if err != nil {
		return nil, fmt.Errorf("could not get branch %s from repository: %w", config.mainBranch, err)
	}
	filesys, err := repo.FS(branch)
	if err != nil {
		return nil, fmt.Errorf("could not open repository filesystem for branch %s: %w", config.mainBranch, err)
	}
	_, err = filesys.Open(config.docsDir)
	if errors.Is(err, fs.ErrNotExist) {
		log.Printf("skipping branch %s: directory %s does not exist", config.mainBranch, config.docsDir)
	} else {
		versions = append([]Version{
			{
				Name:      config.mainBranch,
				Version:   nil,
				IsDefault: prefVersion == preferMainBranch,
				FS:        filesys,
			},
		}, versions...)
	}

	if config.withWorkingDir {
		versions = append([]Version{
			{
				Name:      "dev",
				Version:   nil,
				IsDefault: prefVersion == preferWorkingDir,
				FS:        os.DirFS(config.repositoryDir),
			},
		}, versions...)
	}
	return versions, nil
}
