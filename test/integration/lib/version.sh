#!/usr/bin/env bash

function version_gt() { test "$(echo "$@" | tr " " "n" | sort -V | head -n 1)" != "$1"; }
function version_le() { test "$(echo "$@" | tr " " "n" | sort -V | head -n 1)" == "$1"; }
function version_lt() { test "$(echo "$@" | tr " " "n" | sort -rV | head -n 1)" != "$1"; }
function version_ge() { test "$(echo "$@" | tr " " "n" | sort -rV | head -n 1)" == "$1"; }