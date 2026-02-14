#!/usr/bin/env bash

# Test for issue #2103: HTTP remote cache key should include query parameters.
# Without the fix, two URLs differing only in ?ref= share the same cache and
# the second fetch silently returns stale content from the first.

issue_2103_input_dir="${cases_dir}/issue-2103/input"
issue_2103_tmp_dir=$(mktemp -d)

cleanup_issue_2103() {
  kill "${server_pid}" 2>/dev/null || true
  rm -rf "${issue_2103_tmp_dir}"
  unset HELMFILE_CACHE_HOME HTTP_SERVER_URL
}
trap cleanup_issue_2103 EXIT

test_start "issue-2103: HTTP cache key includes query params"

# --- Start a small HTTP server that returns different YAML per ?ref= ----------

info "Building test HTTP server"
go build -o "${issue_2103_tmp_dir}/server" "${issue_2103_input_dir}/server.go" \
    || fail "Could not build test HTTP server"

"${issue_2103_tmp_dir}/server" > "${issue_2103_tmp_dir}/server_addr.txt" &
server_pid=$!

# Poll until the server writes its address (up to 10 seconds: 20 x 0.5s)
for i in $(seq 1 20); do
    if ! kill -0 "${server_pid}" 2>/dev/null; then
        fail "Test HTTP server failed to start"
    fi
    if [ -s "${issue_2103_tmp_dir}/server_addr.txt" ]; then
        break
    fi
    sleep 0.5
done

if [ ! -s "${issue_2103_tmp_dir}/server_addr.txt" ]; then
    fail "Test HTTP server did not write its address in time"
fi

server_url=$(cat "${issue_2103_tmp_dir}/server_addr.txt")
info "Test HTTP server running at ${server_url}"

# --- Fetch remote values through helmfile and verify cache has both refs ------

export HTTP_SERVER_URL="${server_url}"
export TEST_NS="${test_ns}"
export HELMFILE_CACHE_HOME="${issue_2103_tmp_dir}/cache"

info "Running helmfile template with two releases using different ?ref= values"
${helmfile} -f "${issue_2103_input_dir}/helmfile.yaml.gotmpl" template \
    > "${issue_2103_tmp_dir}/template_output.txt" 2>&1 || {
    helmfile_exit_code=$?
    info "helmfile template output:"
    cat "${issue_2103_tmp_dir}/template_output.txt"
    fail "helmfile template failed with exit code ${helmfile_exit_code}"
}

# Verify that two distinct cache directories were created for the two refs.
# With the fix, the cache key includes the query params so each ref gets its own dir.
# Without the fix, both refs would share one cache dir and the second would be stale.
info "Checking cached files for distinct ref values"

cached_commit1=$(find "${issue_2103_tmp_dir}/cache" -type f -name "raw" -exec grep -l "version: commit1" {} \; || true)
cached_commit2=$(find "${issue_2103_tmp_dir}/cache" -type f -name "raw" -exec grep -l "version: commit2" {} \; || true)

if [ -n "${cached_commit1}" ] && [ -n "${cached_commit2}" ]; then
    info "Found separate cached files:"
    info "  commit1: ${cached_commit1}"
    info "  commit2: ${cached_commit2}"
    if [ "${cached_commit1}" = "${cached_commit2}" ]; then
        fail "Issue #2103 regression: both refs cached to the same file"
    fi
    info "Cache keys are distinct — fix is working"
else
    info "Cache directory contents:"
    find "${issue_2103_tmp_dir}/cache" -type f 2>/dev/null
    info "Template output:"
    cat "${issue_2103_tmp_dir}/template_output.txt"
    fail "Issue #2103 regression: query-param-specific values were not cached separately"
fi

trap - EXIT
test_pass "issue-2103: HTTP cache key includes query params"
