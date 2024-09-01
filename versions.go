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
	Name    string
	Version *semver.Version
	FS      fs.FS
}

func GetDocVersions(repoDir, docsDir, mainBranch string, withWorkingDir bool) ([]Version, error) {
	repo, err := NewGitRepository(repoDir)
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
		_, err = filesys.Open(docsDir)
		if errors.Is(err, fs.ErrNotExist) {
			log.Printf("skipping tag %s: directory %s does not exist", tag.Name(), docsDir)
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

	branch, err := repo.Branch(mainBranch)
	if err != nil {
		return nil, fmt.Errorf("could not get branch %s from repository: %w", mainBranch, err)
	}
	filesys, err := repo.FS(branch)
	if err != nil {
		return nil, fmt.Errorf("could not open repository filesystem for branch %s: %w", mainBranch, err)
	}
	_, err = filesys.Open(docsDir)
	if errors.Is(err, fs.ErrNotExist) {
		log.Printf("skipping branch %s: directory %s does not exist", mainBranch, docsDir)
	} else {
		versions = append([]Version{
			{
				Name:    mainBranch,
				Version: nil,
				FS:      filesys,
			},
		}, versions...)
	}

	if withWorkingDir {
		versions = append([]Version{
			{
				Name:    "dev",
				Version: nil,
				FS:      os.DirFS(repoDir),
			},
		}, versions...)
	}
	return versions, nil
}
