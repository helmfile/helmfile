#!/usr/bin/env bash
# Verify Helm `lookup` works under helmfile for both chartify and non-chartify scenarios.
#
# Assumptions (prepared by test/integration/run.sh):
# - A minikube cluster is running, context=minikube
# - Namespace ${test_ns} is created
# - Variables ${helmfile}, ${kubectl}, and ${cases_dir} are defined

set -euo pipefail

reset_live_secret() {
  info "Resetting live secret to key=init"
  ${kubectl} delete secret my-secret -n ${test_ns}  >/dev/null 2>&1 || true
  init_hf="${cases_dir}/lookup/input-init/helmfile.yaml.gotmpl"
  ${helmfile} -f "${init_hf}" sync -q
}

assert_template_outputs() {
  reset_live_secret
  local hf_file="$1"
  info "Templating ${hf_file} with --template-args=\"--dry-run=server\""
  out=$(mktemp)
  ${helmfile} -f "${hf_file}" template --template-args="--dry-run=server" >"${out}"
  grep -q "key: init" "${out}" || fail "template output did not contain 'key: init'"
  rm -f "${out}"
}

assert_cluster_key_is_init() {
  v=$(${kubectl} get secret my-secret -o jsonpath='{.data.key}' | base64 -d)
  [ "${v}" = "init" ] || fail "expected live secret key to be 'init', got '${v}'"
}

assert_apply() {
  local hf_file="$1"
  reset_live_secret
  info "Applying ${hf_file}"
  ${helmfile} -f "${hf_file}" apply -q || fail "apply failed"
  assert_cluster_key_is_init
}

assert_sync() {
  local hf_file="$1"
  reset_live_secret
  info "Syncing ${hf_file}"
  ${helmfile} -f "${hf_file}" sync -q || fail "sync failed"
  assert_cluster_key_is_init
}

# Non-chartify scenario
plain_hf="${cases_dir}/lookup/input-plain/helmfile.yaml.gotmpl"
assert_template_outputs "${plain_hf}"
assert_apply "${plain_hf}"
assert_sync "${plain_hf}"
test_pass "lookup without chartify"

# Chartify scenario (e.g. forceNamespace triggers chartify)
chartify_hf="${cases_dir}/lookup/input-chartify/helmfile.yaml.gotmpl"
assert_template_outputs "${chartify_hf}"
assert_apply "${chartify_hf}"
assert_sync "${chartify_hf}"
test_pass "lookup with chartify"

