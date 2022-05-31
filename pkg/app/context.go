package app

import (
	"github.com/helmfile/helmfile/pkg/state"
)

type Context struct {
	updatedRepos   map[string]bool
	updatedReposV2 map[string]bool
}

func NewContext() Context {
	return Context{
		updatedRepos:   map[string]bool{},
		updatedReposV2: map[string]bool{},
	}
}

func (ctx Context) SyncReposOnce(st *state.HelmState, helm state.RepoUpdater) error {
	var (
		updated []string
		err     error
	)

	if helm.IsHelm3() {
		updated, err = st.SyncRepos(helm, ctx.updatedRepos)
	} else {
		updated, err = st.SyncRepos(helm, ctx.updatedReposV2)
	}

	for _, r := range updated {
		ctx.updatedRepos[r] = true
	}

	return err
}
