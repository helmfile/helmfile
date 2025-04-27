#!/usr/bin/env bash
# vim: set tabstop=4 shiftwidth=4

set -e
set -o pipefail

# IMPORTS -----------------------------------------------------------------------------------------------------------

# determine working directory to use to relative paths irrespective of starting directory
dir="${BASH_SOURCE%/*}"
if [[ ! -d "${dir}" ]]; then dir="${PWD}"; fi

. "${dir}/lib/output.sh"
. "${dir}/lib/ensure.sh"
. "${dir}/lib/version.sh"


# GLOBALS -----------------------------------------------------------------------------------------------------------

test_ns="helmfile-tests"
helmfile="./helmfile ${EXTRA_HELMFILE_FLAGS} --namespace=${test_ns}"
helm="helm --kube-context=minikube"
kubectl="kubectl --context=minikube --namespace=${test_ns}"
helm_dir="${PWD}/${dir}/.helm"
cases_dir="${dir}/test-cases"
export HELM_DATA_HOME="${helm_dir}/data"
export HELM_HOME="${HELM_DATA_HOME}"
export HELM_PLUGINS="${HELM_DATA_HOME}/plugins"
export HELM_CONFIG_HOME="${helm_dir}/config"
HELM_DIFF_VERSION="${HELM_DIFF_VERSION:-3.11.0}"
HELM_GIT_VERSION="${HELM_GIT_VERSION:-1.3.0}"
HELM_SECRETS_VERSION="${HELM_SECRETS_VERSION:-3.15.0}"
export GNUPGHOME="${PWD}/${dir}/.gnupg"
export SOPS_PGP_FP="B2D6D7BBEC03B2E66571C8C00AD18E16CFDEF700"

# FUNCTIONS ----------------------------------------------------------------------------------------------------------

function wait_deploy_ready() {
    ${kubectl} rollout status deployment ${1}
    while [ "$(${kubectl} get deploy ${1} -o=jsonpath='{.status.readyReplicas}')" == "0" ]; do
        info "Waiting for deployment ${1} to be ready"
        sleep 1
    done
}
function retry() {
    local -r max=${1}
    local -r command=${2}
    n=0
    retry_result=0
    until [ ${n} -ge ${max} ]; do
        info "Executing: ${command} (attempt $((n+1)))"
        ${command} && break  # substitute your command here
        retry_result=$?
        n=$[$n+1]
        # approximated binary exponential backoff to reduce flakiness
        sleep $((n ** 2))
    done
}

function cleanup() {
    set +e
    info "Deleting ${helm_dir}"
    rm -rf ${helm_dir} # remove helm data so reinstalling plugins does not fail
    info "Deleting minikube namespace ${test_ns}"
    $kubectl delete namespace ${test_ns} # remove namespace whenever we exit this script
}

# SETUP --------------------------------------------------------------------------------------------------------------

set -e
trap cleanup EXIT
info "Using namespace: ${test_ns}"
info "Using Helm version:" $(${helm} version --short | grep -o 'v[0-9.]\+')
${helm} plugin ls | grep "^diff" || ${helm} plugin install https://github.com/databus23/helm-diff --version v${HELM_DIFF_VERSION}
${helm} plugin ls | grep "^helm-git" || ${helm} plugin install https://github.com/aslafy-z/helm-git --version v${HELM_GIT_VERSION}
info "Using Kustomize version: $(kustomize version --short | grep -o 'v[0-9.]\+')"
${kubectl} get namespace ${test_ns} &> /dev/null && warn "Namespace ${test_ns} exists, from a previous test run?"
${kubectl} create namespace ${test_ns} || fail "Could not create namespace ${test_ns}"


# TEST CASES----------------------------------------------------------------------------------------------------------

. ${dir}/test-cases/suppress-output-line-regex.sh
. ${dir}/test-cases/chartify-jsonPatches-and-strategicMergePatches.sh
. ${dir}/test-cases/include-template-func.sh
. ${dir}/test-cases/happypath.sh
. ${dir}/test-cases/chartify-with-non-chart-dir.sh
. ${dir}/test-cases/diff-args.sh
. ${dir}/test-cases/helmfile-double-fetch.sh
. ${dir}/test-cases/skip-diff-output.sh
. ${dir}/test-cases/v1-subhelmfile-multi-bases-with-array-values.sh
. ${dir}/test-cases/kustomized-fetch.sh
. ${dir}/test-cases/regression.sh
. ${dir}/test-cases/secretssops.sh
. ${dir}/test-cases/yaml-overwrite.sh
. ${dir}/test-cases/chart-needs.sh
. ${dir}/test-cases/postrender.sh
. ${dir}/test-cases/chartify.sh
. ${dir}/test-cases/deps-mr-1011.sh
. ${dir}/test-cases/deps-kustomization-i-1402.sh
. ${dir}/test-cases/hcl-secrets.sh
. ${dir}/test-cases/issue-1749.sh
. ${dir}/test-cases/issue-1893.sh
. ${dir}/test-cases/state-values-set-cli-args-in-environments.sh

# ALL DONE -----------------------------------------------------------------------------------------------------------

all_tests_passed
