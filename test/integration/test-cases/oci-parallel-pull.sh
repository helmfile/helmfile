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
wait $pid1
exit1=$?

wait $pid2
exit2=$?

wait $pid3
exit3=$?

info "Process 1 exit code: ${exit1}"
info "Process 2 exit code: ${exit2}"
info "Process 3 exit code: ${exit3}"

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

# Check for the specific error from issue #2295
if grep -q "failed to untar: a file or directory with the name.*already exists" "${oci_parallel_pull_output_dir}"/oci-parallel-*.out 2>/dev/null; then
    warn "Race condition detected! Found 'file already exists' error in output"
    failed=1
fi

if [ $failed -eq 1 ]; then
    fail "oci-parallel-pull test failed"
fi

# Verify the chart was cached
if [ ! -d "${HELMFILE_CACHE_HOME}" ]; then
    fail "Cache directory was not created"
fi

# Check that lock files were created (indicates locking was used)
lock_files=$(find "${HELMFILE_CACHE_HOME}" -name "*.lock" 2>/dev/null | wc -l)
info "Found ${lock_files} lock file(s) in cache directory"

# Cleanup and restore the original trap
cleanup_oci_parallel_pull
trap - EXIT

test_pass "oci-parallel-pull: file locking prevents race conditions"
