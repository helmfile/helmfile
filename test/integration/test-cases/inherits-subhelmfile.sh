inherits_input_dir="${cases_dir}/inherits-subhelmfile/input"
inherits_output_tmp=$(mktemp -d)

# Run `helmfile template` on an input file; assert it succeeds and that the
# rendered output contains the given pattern.
expect_template_ok() {
    local desc="$1" file="$2" pattern="$3"
    info "${desc}"
    code=0
    ${helmfile} -f ${inherits_input_dir}/${file} template &> ${inherits_output_tmp}/out.log || code=$?
    if [ ${code} -ne 0 ]; then
        cat ${inherits_output_tmp}/out.log
        fail "${desc}: template should have succeeded, exit=${code}"
    fi
    grep -q "${pattern}" ${inherits_output_tmp}/out.log || { cat ${inherits_output_tmp}/out.log; fail "${desc}: expected '${pattern}' in output"; }
    info "${desc}: OK"
}

# --- Scenario 1: repositories (the issue #1495 regression) -------------------
# Uses the remote incubator/raw chart so that real repository registration is
# exercised — the one thing only an integration test can cover.
test_start "inherits: repositories (issue #1495)"
${helm} repo remove incubator 2>/dev/null || true
expect_template_ok \
    "sub-helmfile resolves chart via repository inherited from parent" \
    "helmfile-inherits.yaml" \
    "kind: ConfigMap"
test_pass "inherits: repositories (issue #1495)"

# --- Scenario 2: environments (resolved values flow to the sub-helmfile) -----
# Uses the local raw chart (no repository needed) to isolate environments
# inheritance. The parent's resolved value must appear in the rendered output.
test_start "inherits: environments"
expect_template_ok \
    "sub-helmfile renders the parent's resolved environment value" \
    "helmfile-env.yaml" \
    "from-parent-env"
test_pass "inherits: environments"

# --- Scenario 3: transitive inheritance (parent -> child -> grandchild) ------
# The env value is inherited across two hops and must reach the grandchild's
# rendered output. Also uses the local raw chart.
test_start "inherits: transitive (parent -> child -> grandchild)"
expect_template_ok \
    "grandchild renders value transitively inherited across two levels" \
    "helmfile-transitive.yaml" \
    "from-parent-env"
test_pass "inherits: transitive (parent -> child -> grandchild)"

# --- Scenario 4: validation (unknown key rejected at parse time) -------------
test_start "inherits: rejects unknown key at parse time"
info "Expecting parse error for unknown inherits key"
code=0
${helmfile} -f ${inherits_input_dir}/helmfile-bad-key.yaml template &> ${inherits_output_tmp}/bad.log || code=$?
if [ ${code} -eq 0 ]; then
    cat ${inherits_output_tmp}/bad.log
    fail "template should have failed for an unknown inherits key, but exited 0"
fi
grep -q "invalid inherits entry" ${inherits_output_tmp}/bad.log || { cat ${inherits_output_tmp}/bad.log; fail "expected 'invalid inherits entry' in the error"; }
grep -q "bogusKey" ${inherits_output_tmp}/bad.log || { cat ${inherits_output_tmp}/bad.log; fail "expected the offending key 'bogusKey' in the error"; }
info "unknown key rejected as expected (exit=${code})"
test_pass "inherits: rejects unknown key at parse time"

# Cleanup so the registered repo does not leak into subsequent tests.
${helm} repo remove incubator 2>/dev/null || true
rm -rf ${inherits_output_tmp}
