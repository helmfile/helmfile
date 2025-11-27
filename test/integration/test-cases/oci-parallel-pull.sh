# Integration test for issue #2295: Cache conflicts running multiple helmfile using the same chart in parallel
# This test verifies that file locking prevents race conditions when multiple helmfile processes
# try to pull the same OCI chart simultaneously.

oci_parallel_pull_case_input_dir="${cases_dir}/oci-parallel-pull/input"
config_file="helmfile.yaml"
oci_parallel_pull_cache_dir=""
oci_parallel_pull_output_dir=""

# Cleanup function for this test
cleanup_oci_parallel_pull() {
  if [ -n "${oci_parallel_pull_cache_dir}" ] && [ -d "${oci_parallel_pull_cache_dir}" ]; then
    rm -rf "${oci_parallel_pull_cache_dir}"
  fi
  if [ -n "${oci_parallel_pull_output_dir}" ] && [ -d "${oci_parallel_pull_output_dir}" ]; then
    rm -rf "${oci_parallel_pull_output_dir}"
  fi
}
trap cleanup_oci_parallel_pull EXIT

test_start "oci-parallel-pull: verify file locking prevents race conditions (issue #2295)"

# Create a temporary cache directory for this test
oci_parallel_pull_cache_dir=$(mktemp -d)
export HELMFILE_CACHE_HOME="${oci_parallel_pull_cache_dir}"

# Create a temporary output directory for test outputs
oci_parallel_pull_output_dir=$(mktemp -d)

info "Using temporary cache directory: ${HELMFILE_CACHE_HOME}"
info "Using temporary output directory: ${oci_parallel_pull_output_dir}"

# Function to check if failure is due to registry issues (not a race condition bug)
is_registry_error() {
    local output_dir="$1"
    # Check for common registry-related errors that are not race condition bugs
    if grep -iqE "(rate limit|too many requests|unauthorized|connection refused|timeout|no such host|i/o timeout)" "${output_dir}"/oci-parallel-*.out 2>/dev/null; then
        return 0  # true - it's a registry error
    fi
    return 1  # false - not a registry error
}

# Run multiple helmfile template commands in parallel using the same chart
# This simulates the scenario described in issue #2295
info "Running 3 parallel helmfile template commands..."

# Start all processes in background
${helmfile} -f ${oci_parallel_pull_case_input_dir}/${config_file} template > "${oci_parallel_pull_output_dir}/oci-parallel-1.out" 2>&1 &
pid1=$!

${helmfile} -f ${oci_parallel_pull_case_input_dir}/${config_file} template > "${oci_parallel_pull_output_dir}/oci-parallel-2.out" 2>&1 &
pid2=$!

${helmfile} -f ${oci_parallel_pull_case_input_dir}/${config_file} template > "${oci_parallel_pull_output_dir}/oci-parallel-3.out" 2>&1 &
pid3=$!

# Wait for all processes and capture exit codes
# Note: We capture exit codes using "|| exit1=$?" pattern to prevent set -e from exiting
# the script when wait returns non-zero (which happens when the process fails)
exit1=0
exit2=0
exit3=0

wait $pid1 || exit1=$?
wait $pid2 || exit2=$?
wait $pid3 || exit3=$?

info "Process 1 exit code: ${exit1}"
info "Process 2 exit code: ${exit2}"
info "Process 3 exit code: ${exit3}"

# Check for the specific error from issue #2295 (race condition bug)
# Use case-insensitive extended regex to catch variations from different tar/helm versions
if grep -iqE "(failed to untar.*already exists|file or directory.*already exists|a file.*already exists)" "${oci_parallel_pull_output_dir}"/oci-parallel-*.out 2>/dev/null; then
    warn "Race condition detected! Found 'file already exists' error in output"
    cat "${oci_parallel_pull_output_dir}"/oci-parallel-*.out
    fail "oci-parallel-pull test failed due to race condition (issue #2295)"
fi

# Check for failures
failed=0
if [ $exit1 -ne 0 ]; then
    warn "Process 1 failed:"
    cat "${oci_parallel_pull_output_dir}/oci-parallel-1.out"
    failed=1
fi

if [ $exit2 -ne 0 ]; then
    warn "Process 2 failed:"
    cat "${oci_parallel_pull_output_dir}/oci-parallel-2.out"
    failed=1
fi

if [ $exit3 -ne 0 ]; then
    warn "Process 3 failed:"
    cat "${oci_parallel_pull_output_dir}/oci-parallel-3.out"
    failed=1
fi

if [ $failed -eq 1 ]; then
    # Check if this is a registry error (rate limit, network issue, etc.)
    # These are not bugs in helmfile, so we should skip the test rather than fail
    if is_registry_error "${oci_parallel_pull_output_dir}"; then
        warn "Test skipped due to external registry issues (rate limit, network, etc.)"
        warn "This is not a helmfile bug - the file locking mechanism cannot be tested"
        # Clean up and exit successfully to not fail CI on external issues
        cleanup_oci_parallel_pull
        trap - EXIT
        test_pass "oci-parallel-pull: skipped due to external registry issues"
        return 0 2>/dev/null || exit 0
    fi
    fail "oci-parallel-pull test failed"
fi

# Verify the chart was cached
if [ ! -d "${HELMFILE_CACHE_HOME}" ]; then
    fail "Cache directory was not created"
fi

# Check that lock files were created (indicates locking was used)
lock_files=$(find "${HELMFILE_CACHE_HOME}" -name "*.lock" 2>/dev/null | wc -l)
info "Found ${lock_files} lock file(s) in cache directory"
if [ "${lock_files}" -lt 1 ]; then
    warn "No lock files found - locking may not have been used"
fi

# Cleanup and restore the original trap
cleanup_oci_parallel_pull
trap - EXIT

test_pass "oci-parallel-pull: file locking prevents race conditions"
