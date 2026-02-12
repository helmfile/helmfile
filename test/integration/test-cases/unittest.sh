unittest_input_dir="${cases_dir}/unittest/input"
helmfile_real="$(pwd)/${helmfile}"
HELM_UNITTEST_VERSION="${HELM_UNITTEST_VERSION:-1.0.3}"

# Ensure helm-unittest plugin is installed (matching plugin install pattern from run.sh)
info "Ensuring helm-unittest plugin v${HELM_UNITTEST_VERSION} is installed"
${helm} plugin ls | grep "^unittest" || ${helm} plugin install https://github.com/helm-unittest/helm-unittest --version v${HELM_UNITTEST_VERSION} ${PLUGIN_INSTALL_FLAGS} || fail "Could not install helm-unittest plugin"

test_start "helmfile unittest - runs unit tests on releases with unitTests defined"
cd "${unittest_input_dir}"
${helmfile_real} unittest || fail "helmfile unittest should succeed"
cd -
test_pass "helmfile unittest - runs unit tests on releases with unitTests defined"

test_start "helmfile unittest - with selector targeting release without unitTests"
cd "${unittest_input_dir}"
${helmfile_real} -l name=no-tests-app unittest || fail "helmfile unittest should succeed for releases without unitTests (skips them)"
cd -
test_pass "helmfile unittest - with selector targeting release without unitTests"

test_start "helmfile unittest - with selector targeting release with unitTests"
cd "${unittest_input_dir}"
${helmfile_real} -l name=test-app unittest || fail "helmfile unittest should succeed for releases with unitTests"
cd -
test_pass "helmfile unittest - with selector targeting release with unitTests"
