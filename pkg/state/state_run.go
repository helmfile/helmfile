package state

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/variantdev/dag/pkg/dag"

	"github.com/helmfile/helmfile/pkg/helmexec"
)

type result struct {
	release ReleaseSpec
	err     error
}

func (st *HelmState) scatterGather(concurrency int, items int, produceInputs func(), receiveInputsAndProduceIntermediates func(int), aggregateIntermediates func()) {
	if concurrency < 1 || concurrency > items {
		concurrency = items
	}

	// WaitGroup is required to wait until goroutine per job in job queue cleanly stops.
	var waitGroup sync.WaitGroup
	waitGroup.Add(concurrency)

	go produceInputs()

	for w := 1; w <= concurrency; w++ {
		go func(id int) {
			receiveInputsAndProduceIntermediates(id)
			waitGroup.Done()
		}(w)
	}

	aggregateIntermediates()

	// Wait until all the goroutines to gracefully finish
	waitGroup.Wait()
}

func (st *HelmState) scatterGatherReleases(helm helmexec.Interface, concurrency int,
	do func(ReleaseSpec, int) error) []error {
	return st.iterateOnReleases(helm, concurrency, st.Releases, do)
}

// nolint: unparam
func (st *HelmState) iterateOnReleases(helm helmexec.Interface, concurrency int, inputs []ReleaseSpec,
	do func(ReleaseSpec, int) error) []error {
	var errs []error

	inputsSize := len(inputs)

	releases := make(chan ReleaseSpec)
	results := make(chan result)

	st.scatterGather(
		concurrency,
		inputsSize,
		func() {
			for _, release := range inputs {
				releases <- release
			}
			close(releases)
		},
		func(id int) {
			for release := range releases {
				err := do(release, id)
				st.logger.Debugf("release %q processed", release.Name)
				results <- result{release: release, err: err}
			}
		},
		func() {
			for range inputs {
				r := <-results
				if r.err != nil {
					errs = append(errs, fmt.Errorf("release \"%s\" failed: %v", r.release.Name, r.err))
				}
			}
		},
	)

	if len(errs) != 0 {
		return errs
	}

	return nil
}

type PlanOptions struct {
	Purpose                string
	Reverse                bool
	IncludeNeeds           bool
	IncludeTransitiveNeeds bool
	SkipNeeds              bool
	SelectedReleases       []ReleaseSpec
}

func (st *HelmState) PlanReleases(opts PlanOptions) ([][]ReleaseSpec, error) {
	marked, err := st.GetSelectedReleases(opts.IncludeTransitiveNeeds)
	if err != nil {
		return nil, err
	}

	groups, err := SortedReleaseGroups(marked, opts)
	if err != nil {
		return nil, err
	}

	return groups, nil
}

func SortedReleaseGroups(releases []ReleaseSpec, opts PlanOptions) ([][]ReleaseSpec, error) {
	reverse := opts.Reverse

	groups, err := GroupReleasesByDependency(releases, opts)
	if err != nil {
		return nil, err
	}

	if reverse {
		for i, j := 0, len(groups)-1; i < j; i, j = i+1, j-1 {
			groups[i], groups[j] = groups[j], groups[i]
		}
	}

	return groups, nil
}

func GroupReleasesByDependency(releases []ReleaseSpec, opts PlanOptions) ([][]ReleaseSpec, error) {
	idToReleases := map[string][]ReleaseSpec{}
	idToIndex := map[string]int{}

	d := dag.New()
	for i, r := range releases {
		id := ReleaseToID(&r)

		idToReleases[id] = append(idToReleases[id], r)
		idToIndex[id] = i

		var needs []string
		for i := 0; i < len(r.Needs); i++ {
			n := r.Needs[i]
			needs = append(needs, n)
		}
		d.Add(id, dag.Dependencies(needs))
	}

	var ids []string
	for id := range idToReleases {
		ids = append(ids, id)
	}

	var selectedReleaseIDs []string

	for _, r := range opts.SelectedReleases {
		release := r
		id := ReleaseToID(&release)
		selectedReleaseIDs = append(selectedReleaseIDs, id)
	}

	plan, err := d.Plan(dag.SortOptions{
		Only:                selectedReleaseIDs,
		WithDependencies:    opts.IncludeNeeds,
		WithoutDependencies: opts.SkipNeeds,
	})
	if err != nil {
		if ude, ok := err.(*dag.UnhandledDependencyError); ok {
			msgs := make([]string, len(ude.UnhandledDependencies))
			for i, ud := range ude.UnhandledDependencies {
				id := ud.Id

				ds := make([]string, len(ud.Dependents))
				for i, d := range ud.Dependents {
					ds[i] = fmt.Sprintf("%q", d)
				}

				var dsHumanized string
				if len(ds) < 3 {
					dsHumanized = strings.Join(ds, " and ")
				} else {
					dsHumanized = strings.Join(ds[:len(ds)-1], ", ")
					dsHumanized += ", and " + ds[len(ds)-1]
				}

				var verb string
				if len(ds) == 1 {
					verb = "depends"
				} else {
					verb = "depend"
				}

				idComponents := strings.Split(id, "/")
				name := idComponents[len(idComponents)-1]

				msg := fmt.Sprintf(
					"release %s %s on %q which does not match the selectors. "+
						"Please add a selector like \"--selector name=%s\", or indicate whether to skip (--skip-needs) or include (--include-needs) these dependencies",
					dsHumanized,
					verb,
					id,
					name,
				)
				msgs[i] = msg
			}
			return nil, errors.New(msgs[0])
		} else if ude, ok := err.(*dag.UndefinedDependencyError); ok {
			var quotedReleaseNames []string
			for _, d := range ude.Dependents {
				quotedReleaseNames = append(quotedReleaseNames, fmt.Sprintf("%q", d))
			}

			idComponents := strings.Split(ude.UndefinedNode, "/")
			name := idComponents[len(idComponents)-1]
			humanReadableUndefinedReleaseInfo := fmt.Sprintf(`named %q with appropriate "namespace" and "kubeContext"`, name)

			return nil, fmt.Errorf(
				`release(s) %s depend(s) on an undefined release %q. Perhaps you made a typo in "needs" or forgot defining a release %s?`,
				strings.Join(quotedReleaseNames, ", "),
				ude.UndefinedNode,
				humanReadableUndefinedReleaseInfo,
			)
		}
		return nil, err
	}

	var result [][]ReleaseSpec

	for groupIndex := 0; groupIndex < len(plan); groupIndex++ {
		dagNodesInGroup := plan[groupIndex]

		var idsInGroup []string
		var releasesInGroup []ReleaseSpec

		for _, node := range dagNodesInGroup {
			idsInGroup = append(idsInGroup, node.Id)
		}

		// Make the helmfile behavior deterministic for reproducibility and ease of testing
		// We try to keep the order of definitions to keep backward-compatibility
		// See https://github.com/roboll/helmfile/issues/988
		sort.Slice(idsInGroup, func(i, j int) bool {
			ii := idToIndex[idsInGroup[i]]
			ij := idToIndex[idsInGroup[j]]
			return ii < ij
		})

		for _, id := range idsInGroup {
			rs, ok := idToReleases[id]
			if !ok {
				panic(fmt.Errorf("bug: unexpectedly failed to get releases for id %q: %v", id, ids))
			}
			releasesInGroup = append(releasesInGroup, rs...)
		}

		result = append(result, releasesInGroup)
	}

	return result, nil
}
