#!/usr/bin/env bash

test_start "kubedog-tracking - resource tracking with kubedog integration"

kubedog_case_dir="${cases_dir}/kubedog-tracking"
config_file="helmfile.yaml.gotmpl"

info "Testing kubedog integration with httpbin chart"

# Test 1: Basic sync with kubedog tracking
info "Syncing release with basic kubedog tracking"
${helmfile} -f "${kubedog_case_dir}/${config_file}" -l name=httpbin-basic sync
code=$?
[ "${code}" -eq 0 ] || fail "unexpected exit code returned by helmfile sync: ${code}"

wait_deploy_ready httpbin-basic-httpbin
info "Verifying httpbin-basic deployment is running"
${kubectl} get deployment httpbin-basic-httpbin -n "${test_ns}" || fail "httpbin-basic deployment not found"

# Test 2: Sync with whitelist filtering
info "Syncing release with whitelist filtering"
${helmfile} -f "${kubedog_case_dir}/${config_file}" -l name=httpbin-with-whitelist sync
code=$?
[ "${code}" -eq 0 ] || fail "unexpected exit code returned by helmfile sync with whitelist: ${code}"

wait_deploy_ready httpbin-with-whitelist-httpbin
info "Verifying httpbin-with-whitelist deployment is running"
${kubectl} get deployment httpbin-with-whitelist-httpbin -n "${test_ns}" || fail "httpbin-with-whitelist deployment not found"

# Test 3: Sync with specific resource tracking
info "Syncing release with specific resource tracking"
${helmfile} -f "${kubedog_case_dir}/${config_file}" -l name=httpbin-with-resources sync
code=$?
[ "${code}" -eq 0 ] || fail "unexpected exit code returned by helmfile sync with resource tracking: ${code}"

wait_deploy_ready httpbin-with-resources-httpbin
info "Verifying httpbin-with-resources deployment is running"
${kubectl} get deployment httpbin-with-resources-httpbin -n "${test_ns}" || fail "httpbin-with-resources deployment not found"

# Test 4: Apply all releases with kubedog via CLI flag
info "Testing apply with kubedog CLI flag"
${helmfile} -f "${kubedog_case_dir}/${config_file}" apply --track-mode kubedog --track-timeout 60
code=$?
[ "${code}" -eq 0 ] || fail "unexpected exit code returned by helmfile apply: ${code}"

# Test 5: Cleanup
info "Destroying all releases"
${helmfile} -f "${kubedog_case_dir}/${config_file}" destroy
code=$?
[ "${code}" -eq 0 ] || fail "unexpected exit code returned by helmfile destroy: ${code}"

info "kubedog integration test completed successfully"

test_pass "kubedog-tracking"
