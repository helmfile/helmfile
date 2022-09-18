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


# GLOBALS -----------------------------------------------------------------------------------------------------------

test_ns="helmfile-tests-$(date +"%Y%m%d-%H%M%S")"
helmfile="./helmfile ${EXTRA_HELMFILE_FLAGS} --namespace=${test_ns}"
helm="helm --kube-context=minikube"
kubectl="kubectl --context=minikube --namespace=${test_ns}"
helm_dir="${PWD}/${dir}/.helm"
export HELM_DATA_HOME="${helm_dir}/data"
export HELM_HOME="${HELM_DATA_HOME}"
export HELM_PLUGINS="${HELM_DATA_HOME}/plugins"
export HELM_CONFIG_HOME="${helm_dir}/config"
HELM_DIFF_VERSION=3.3.1
HELM_SECRETS_DEFAULT_VERSION=3.15.0
HELM_SECRETS_VERSION="${HELM_SECRETS_VERSION:-$HELM_SECRETS_DEFAULT_VERSION}"
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
# helm v2
if helm version --client 2>/dev/null | grep '"v2\.'; then
    helm_major_version=2
    info "Using Helm version: $(helm version --short --client | grep -o v.*$)"
    ${helm} init --stable-repo-url https://charts.helm.sh/stable --wait --override spec.template.spec.automountServiceAccountToken=true
else # helm v3
    helm_major_version=3
    info "Using Helm version: $(helm version --short | grep -o v.*$)"
fi
${helm} plugin ls | grep diff || ${helm} plugin install https://github.com/databus23/helm-diff --version v${HELM_DIFF_VERSION}
info "Using Kustomize version: $(kustomize version --short | grep -o 'v[0-9.]\+')"
${kubectl} get namespace ${test_ns} &> /dev/null && warn "Namespace ${test_ns} exists, from a previous test run?"
$kubectl create namespace ${test_ns} || fail "Could not create namespace ${test_ns}"


# TEST CASES----------------------------------------------------------------------------------------------------------

test_start "happypath - simple rollout of httpbin chart"

info "Diffing ${dir}/happypath.yaml"
bash -c "${helmfile} -f ${dir}/happypath.yaml diff --detailed-exitcode; code="'$?'"; [ "'${code}'" -eq 2 ]" || fail "unexpected exit code returned by helmfile diff"

info "Diffing ${dir}/happypath.yaml without color"
bash -c "${helmfile} -f ${dir}/happypath.yaml --no-color diff --detailed-exitcode; code="'$?'"; [ "'${code}'" -eq 2 ]" || fail "unexpected exit code returned by helmfile diff"

info "Diffing ${dir}/happypath.yaml with limited context"
bash -c "${helmfile} -f ${dir}/happypath.yaml diff --context 3 --detailed-exitcode; code="'$?'"; [ "'${code}'" -eq 2 ]" || fail "unexpected exit code returned by helmfile diff"

info "Diffing ${dir}/happypath.yaml with altered output"
bash -c "${helmfile} -f ${dir}/happypath.yaml diff --output simple --detailed-exitcode; code="'$?'"; [ "'${code}'" -eq 2 ]" || fail "unexpected exit code returned by helmfile diff"

info "Templating ${dir}/happypath.yaml"
rm -rf ${dir}/tmp
${helmfile} -f ${dir}/happypath.yaml --debug template --output-dir tmp
code=$?
[ ${code} -eq 0 ] || fail "unexpected exit code returned by helmfile template: ${code}"
for output in $(ls -d ${dir}/tmp/*); do
    # e.g. test/integration/tmp/happypath-877c0dd4-helmx/helmx
    for release_dir in $(ls -d ${output}/*); do
        release_name=$(basename ${release_dir})
        golden_dir=${dir}/templates-golden/v${helm_major_version}/${release_name}
        info "Comparing template output ${release_dir}/templates with ${golden_dir}"
        ./diff-yamls ${golden_dir} ${release_dir}/templates || fail "unexpected diff in template result for ${release_name}"
    done
done

info "Applying ${dir}/happypath.yaml"
bash -c "${helmfile} -f ${dir}/happypath.yaml apply --detailed-exitcode; code="'$?'"; echo Code: "'$code'"; [ "'${code}'" -eq 2 ]" || fail "unexpected exit code returned by helmfile apply"

info "Syncing ${dir}/happypath.yaml"
${helmfile} -f ${dir}/happypath.yaml sync
wait_deploy_ready httpbin-httpbin
retry 5 "curl --fail $(minikube service --url --namespace=${test_ns} httpbin-httpbin)/status/200"
[ ${retry_result} -eq 0 ] || fail "httpbin failed to return 200 OK"

info "Applying ${dir}/happypath.yaml"
${helmfile} -f ${dir}/happypath.yaml apply --detailed-exitcode
code=$?
[ ${code} -eq 0 ] || fail "unexpected exit code returned by helmfile apply: want 0, got ${code}"

info "Locking dependencies"
${helmfile} -f ${dir}/happypath.yaml deps
code=$?
[ ${code} -eq 0 ] || fail "unexpected exit code returned by helmfile deps: ${code}"

info "Applying ${dir}/happypath.yaml with locked dependencies"
${helmfile} -f ${dir}/happypath.yaml apply
code=$?
[ ${code} -eq 0 ] || fail "unexpected exit code returned by helmfile apply: ${code}"
${helm} list --namespace=${test_ns} || fail "unable to list releases"

info "Deleting release"
${helmfile} -f ${dir}/happypath.yaml delete
${helm} status --namespace=${test_ns} httpbin &> /dev/null && fail "release should not exist anymore after a delete"

info "Ensuring \"helmfile delete\" doesn't fail when no releases installed"
${helmfile} -f ${dir}/happypath.yaml delete || fail "\"helmfile delete\" shouldn't fail when there are no installed releases"

info "Ensuring \"helmfile template\" output does contain only YAML docs"
(${helmfile} -f ${dir}/happypath.yaml template | kubectl apply -f -) || fail "\"helmfile template | kubectl apply -f -\" shouldn't fail"

test_pass "happypath"

test_start "regression tests"

if [[ helm_major_version -eq 3 ]]; then
  info "https://github.com/roboll/helmfile/issues/1857"
  (${helmfile} -f ${dir}/issue.1857.yaml --state-values-set grafanaEnabled=true template | grep grafana 1>/dev/null) || fail "\"helmfile template\" shouldn't include grafana"
  ! (${helmfile} -f ${dir}/issue.1857.yaml --state-values-set grafanaEnabled=false template | grep grafana) || fail "\"helmfile template\" shouldn't include grafana"

  info "https://github.com/roboll/helmfile/issues/1867"
  (${helmfile} -f ${dir}/issue.1867.yaml template 1>/dev/null) || fail "\"helmfile template\" shouldn't fail"

  info "https://github.com/roboll/helmfile/issues/2118"
  (${helmfile} -f ${dir}/issue.2118.yaml template 1>/dev/null) || fail "\"helmfile template\" shouldn't fail"
else
  info "There are no regression tests for helm 2 because all the target charts have dropped helm 2 support."
fi

test_pass "regression tests"

if [[ helm_major_version -eq 3 ]]; then
  export VAULT_ADDR=http://127.0.0.1:8200
  export VAULT_TOKEN=toor
  sops="sops --hc-vault-transit $VAULT_ADDR/v1/sops/keys/key"
  mkdir -p ${dir}/tmp

  info "Encrypt secrets"
  ${sops} -e ${dir}/env-1.secrets.yaml > ${dir}/tmp/env-1.secrets.sops.yaml || fail "${sops} failed at ${dir}/env-1.secrets.yaml"
  ${sops} -e ${dir}/env-2.secrets.yaml > ${dir}/tmp/env-2.secrets.sops.yaml || fail "${sops} failed at ${dir}/env-2.secrets.yaml"

  test_start "secretssops.1 - should fail without secrets plugin"

  info "Ensure helm-secrets is not installed"
  ${helm} plugin rm secrets || true

  info "Ensure helmfile fails when no helm-secrets is installed"
  unset code
  ${helmfile} -f ${dir}/secretssops.yaml -e direct build || code="$?"; code="${code:-0}"
  echo Code: "${code}"
  [ "${code}" -ne 0 ] || fail "\"helmfile build\" should fail without secrets plugin"

  test_pass "secretssops.1"

  test_start "secretssops.2 - should succeed with secrets plugin"

  info "Ensure helm-secrets is installed"
  ${helm} plugin install https://github.com/jkroepke/helm-secrets --version v${HELM_SECRETS_VERSION}

  info "Ensure helmfile succeed when helm-secrets is installed"
  ${helmfile} -f ${dir}/secretssops.yaml -e direct build || fail "\"helmfile build\" shouldn't fail"

  test_pass "secretssops.2"

    test_start "secretssops.3 - should order secrets correctly"

    tmp=$(mktemp -d)
    direct=${tmp}/direct.build.yaml
    reverse=${tmp}/reverse.build.yaml
    golden_dir=${dir}/secrets-golden

    info "Building secrets output"

    info "Comparing build/direct output ${direct} with ${golden_dir}"
    for i in $(seq 10); do
        info "Comparing build/direct #$i"
        ${helmfile} -f ${dir}/secretssops.yaml -e direct template --skip-deps > ${direct} || fail "\"helmfile template\" shouldn't fail"
        ./yamldiff ${golden_dir}/direct.build.yaml ${direct} || fail "\"helmfile template\" should be consistent"
        echo code=$?
    done

    info "Comparing build/reverse output ${direct} with ${golden_dir}"
    for i in $(seq 10); do
        info "Comparing build/reverse #$i"
        ${helmfile} -f ${dir}/secretssops.yaml -e reverse template --skip-deps > ${reverse} || fail "\"helmfile template\" shouldn't fail"
        ./yamldiff ${golden_dir}/reverse.build.yaml ${reverse} || fail "\"helmfile template\" should be consistent"
        echo code=$?
    done

    test_pass "secretssops.3"

fi

# ALL DONE -----------------------------------------------------------------------------------------------------------

all_tests_passed
