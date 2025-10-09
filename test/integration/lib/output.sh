#!/usr/bin/env bash

declare -i tests_total=0

function info () {
    tput bold >/dev/null 2>&1 || true; tput setaf 4 >/dev/null 2>&1 || true; echo -n "INFO: "; tput sgr0 >/dev/null 2>&1 || true; echo "${@}"
}
function warn () {
    tput bold >/dev/null 2>&1 || true; tput setaf 3 >/dev/null 2>&1 || true; echo -n "WARN: "; tput sgr0 >/dev/null 2>&1 || true; echo "${@}"
}
function fail () {
    tput bold >/dev/null 2>&1 || true; tput setaf 1 >/dev/null 2>&1 || true; echo -n "FAIL: "; tput sgr0 >/dev/null 2>&1 || true; echo "${@}"
    exit 1
}
function test_start () {
    tput bold >/dev/null 2>&1 || true; tput setaf 6 >/dev/null 2>&1 || true; echo -n "TEST: "; tput sgr0 >/dev/null 2>&1 || true; echo "${@}"
}
function test_pass () {
    tests_total=$((tests_total+1))
    tput bold >/dev/null 2>&1 || true; tput setaf 2 >/dev/null 2>&1 || true; echo -n "PASS: "; tput sgr0 >/dev/null 2>&1 || true; echo "${@}"
}
function all_tests_passed () {
    tput bold >/dev/null 2>&1 || true; tput setaf 2 >/dev/null 2>&1 || true; echo -n "PASS: "; tput sgr0 >/dev/null 2>&1 || true; echo "${tests_total} tests passed"
}
