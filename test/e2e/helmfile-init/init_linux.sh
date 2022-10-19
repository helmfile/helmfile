#!/usr/bin/env bash
# vim: set tabstop=4 shiftwidth=4

set -e
set -o pipefail

# IMPORTS -----------------------------------------------------------------------------------------------------------

# determine working directory to use to relative paths irrespective of starting directory
dir="${BASH_SOURCE%/*}"
if [[ ! -d "${dir}" ]]; then dir="${PWD}"; fi

. "${dir}/../../integration/lib/output.sh"

helmfile="./helmfile"
helm_dir="${PWD}/${dir}/.helm"
helm=`which helm`
export HELM_DATA_HOME="${helm_dir}/data"
export HELM_HOME="${HELM_DATA_HOME}"
export HELM_PLUGINS="${HELM_DATA_HOME}/plugins"
export HELM_CONFIG_HOME="${helm_dir}/config"

function cleanup() {
    set +e
    info "Deleting ${helm_dir}"
    rm -rf ${helm_dir} # remove helm data so reinstalling plugins does not fail
}

function removehelm() {
  [ -f $helm ] && rm -rf $helm
}

set -e
trap cleanup EXIT

removehelm

expect <<EOF
set timeout -1
spawn ${helmfile} init
expect {
    "*y/n" {send "y\r";exp_continue}
    eof
}
EOF

helm plugin ls | grep diff || fail "helmfile init run fail"

all_tests_passed