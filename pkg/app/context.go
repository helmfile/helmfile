package app

import (
	"github.com/helmfile/helmfile/pkg/state"
)

type Context struct {
	updatedRepos map[string]bool
}

func NewContext() Context {
	return Context{
		updatedRepos: map[string]bool{},
	}
}

func (ctx Context) SyncReposOnce(st *state.HelmState, helm state.RepoUpdater) error {
	updated, err := st.SyncRepos(helm, ctx.updatedRepos)

	for _, r := range updated {
		ctx.updatedRepos[r] = true
	}

	return err
}
