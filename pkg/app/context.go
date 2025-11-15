package app

import (
	"sync"

	"github.com/helmfile/helmfile/pkg/state"
)

type Context struct {
	updatedRepos map[string]bool
	mu           sync.Mutex
}

func NewContext() Context {
	return Context{
		updatedRepos: make(map[string]bool),
	}
}

func (ctx *Context) SyncReposOnce(st *state.HelmState, helm state.RepoUpdater) error {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	updated, err := st.SyncRepos(helm, ctx.updatedRepos)

	for _, r := range updated {
		ctx.updatedRepos[r] = true
	}

	return err
}
