package main

import (
	"fmt"
	"io/fs"
	"sort"

	"github.com/MarkKremer/gopxl-docs/internal/gitfs"
	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/pkg/errors"
)

type Repository interface {
	Versions() ([]Version, error)
	FS(version Version) (fs.FS, error)
}

type Version interface {
	DisplayName() string
}

type GitRepository struct {
	path       string
	repository *git.Repository
}

type GitVersion struct {
	v   *semver.Version
	ref *plumbing.Reference
}

func (g *GitVersion) DisplayName() string {
	if g.v == nil {
		return g.ref.Name().Short()
	} else {
		return fmt.Sprintf("v%d.x", g.v.Major())
	}
}

func NewGitRepository(path string) (*GitRepository, error) {
	repository, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("could not open Git repository %s: %w", path, err)
	}

	return &GitRepository{
		path:       path,
		repository: repository,
	}, nil
}

func (gr *GitRepository) Versions() ([]Version, error) {
	ts, err := gr.repository.Tags()
	if err != nil {
		return nil, fmt.Errorf("could not read Git tags: %w", err)
	}
	// Get the newest tag of each major version.
	majors := make(map[uint64]*GitVersion)
	err = ts.ForEach(func(reference *plumbing.Reference) error {
		v, err := semver.NewVersion(reference.Name().Short())
		if err != nil {
			return fmt.Errorf("could not parse tag %s as version: %w", reference.Name().Short(), err)
		}
		// Store version if there is no newer version stored.
		if other, ok := majors[v.Major()]; !ok || other.v.LessThan(v) {
			majors[v.Major()] = &GitVersion{
				v:   v,
				ref: reference,
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not iterate Git tags: %w", err)
	}

	var versions []Version
	for _, v := range majors {
		versions = append(versions, v)
	}
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].(*GitVersion).v.LessThan(versions[j].(*GitVersion).v)
	})

	// Get either the main branch or head.
	head, err := gr.repository.Head()
	if err != nil {
		return nil, fmt.Errorf("could not get HEAD commit: %w", err)
	}
	versions = append(versions, &GitVersion{
		v:   nil,
		ref: head,
	})

	return versions, nil
}

func (gr *GitRepository) FS(version Version) (fs.FS, error) {
	gv, ok := version.(*GitVersion)
	if !ok {
		return nil, errors.New("the provided version is not a GitVersion")
	}
	obj, err := gr.repository.CommitObject(gv.ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("could not get commit object from repository: %w", err)
	}

	return gitfs.NewGitFs(obj)
}
