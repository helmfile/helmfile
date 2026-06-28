# Integration test for issue #821: go-getter URL in ad-hoc dependencies
# https://github.com/helmfile/helmfile/issues/821
#
# Before the fix, an ad-hoc dependency using a go-getter URL like
#   git::https://github.com/org/repo.git@path?ref=tag
# was passed to chartify as-is, which then tried to resolve it via
# `helm repo list` and failed with:
#   "failed reading adhoc dependencies: no helm list entry found for repository"
#
# This test commits a chart to a throwaway local git repo and references it via
# a "git::file://..." go-getter URL. Using file:// (instead of https://) keeps
# the test deterministic and network-free while exercising the exact fix path
# (remote.IsRemote + downloadAdhocDepChartWithGoGetter in PrepareChartify).

issue_821_case_dir="${cases_dir}/issue-821-adhoc-dep-go-getter"
issue_821_tmp=""

cleanup_issue_821() {
    if [ -n "${issue_821_tmp}" ] && [ -d "${issue_821_tmp}" ]; then
        rm -rf "${issue_821_tmp}"
    fi
}
trap cleanup_issue_821 EXIT

issue_821_tmp=$(mktemp -d)

test_start "issue #821: go-getter URL in ad-hoc dependencies"

info "Setting up throwaway git repo for the ad-hoc dependency chart"

# Build a git repo containing the dependency chart at charts/crd, mirroring the
# issue's "@<subdir>" go-getter syntax.
dep_repo="${issue_821_tmp}/dep-repo"
mkdir -p "${dep_repo}/charts/crd"
cp -R "${issue_821_case_dir}/input/dep-chart/." "${dep_repo}/charts/crd/"

git -C "${dep_repo}" init -q -b main
git -C "${dep_repo}" config user.email "test@example.com"
git -C "${dep_repo}" config user.name "helmfile-tests"
git -C "${dep_repo}" add .
git -C "${dep_repo}" commit -q -m "issue-821 dep chart"

# Render the helmfile with absolute paths so it is independent of CWD.
cat > "${issue_821_tmp}/helmfile.yaml" <<EOF
releases:
  - name: issue-821
    namespace: ${test_ns}
    chart: "${issue_821_case_dir}/input/main-chart"
    dependencies:
      - chart: "git::file://${dep_repo}@charts/crd?ref=main"
        version: "0.1.x"
EOF

issue_821_out="${issue_821_tmp}/out.yaml"

info "Running helmfile template with a go-getter ad-hoc dependency"
${helmfile} -f "${issue_821_tmp}/helmfile.yaml" template > "${issue_821_out}" 2>&1
issue_821_rc=$?

if [ ${issue_821_rc} -ne 0 ]; then
    cat "${issue_821_out}"
    fail "helmfile template should not fail for a go-getter ad-hoc dependency"
fi

# Guard against the pre-fix regression: the old error must never appear.
if grep -q "no helm list entry found for repository" "${issue_821_out}"; then
    cat "${issue_821_out}"
    fail "output should not contain the pre-fix 'no helm list entry found' error"
fi

# The go-getter-fetched dependency must be materialized and rendered: its
# ConfigMap appears in the template output alongside the main chart's.
if ! grep -q "issue-821-adhoc-dep-cm" "${issue_821_out}"; then
    cat "${issue_821_out}"
    fail "expected the go-getter ad-hoc dependency resource 'issue-821-adhoc-dep-cm' in the template output"
fi

if ! grep -q "issue-821-main-cm" "${issue_821_out}"; then
    cat "${issue_821_out}"
    fail "expected the main chart resource 'issue-821-main-cm' in the template output"
fi

info "go-getter ad-hoc dependency was fetched and rendered correctly"

cleanup_issue_821
trap - EXIT

test_pass "issue #821: go-getter URL in ad-hoc dependencies"
