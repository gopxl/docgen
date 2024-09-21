package main

import (
	"fmt"
	"io/fs"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/gopxl/docgen/internal/gitfs"
)

type GitRepository struct {
	path       string
	repository *git.Repository
}

type GitReference struct {
	ref *plumbing.Reference
}

func (g *GitReference) Name() string {
	return g.ref.Name().Short()
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

func (gr *GitRepository) Branch(name string) (*GitReference, error) {
	branch, err := gr.repository.Branch(name)
	if err != nil {
		return nil, fmt.Errorf("could not read Git branch %s: %w", name, err)
	}
	ref, err := gr.repository.Reference(branch.Merge, true)
	if err != nil {
		return nil, fmt.Errorf("could not get reference for branch %s: %w", name, err)
	}
	return &GitReference{
		ref: ref,
	}, nil
}

func (gr *GitRepository) Tags() ([]*GitReference, error) {
	tags, err := gr.repository.Tags()
	if err != nil {
		return nil, fmt.Errorf("could not read Git tags: %w", err)
	}
	var ts []*GitReference
	err = tags.ForEach(func(reference *plumbing.Reference) error {
		ts = append(ts, &GitReference{
			ref: reference,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error iterating Git tags: %w", err)
	}
	return ts, nil
}

func (gr *GitRepository) FS(tag *GitReference) (fs.FS, error) {
	obj, err := gr.repository.CommitObject(tag.ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("could not get commit object from repository: %w", err)
	}

	return gitfs.NewGitFs(obj)
}
