#!/usr/bin/env bash

# Check environment is correctly setup

if ! hash minikube 2>/dev/null; then
    fail "Minikube needs to be installed."
fi
if [ ! $(minikube status --format '{{.MinikubeStatus}}') == "Running" ]; then
    fail "Minikube is not running."
fi
if [ ! $(minikube status --format '{{.ClusterStatus}}') == "Running" ]; then
    fail "Minikube Cluster is not running."
fi
if ! kubectl version 1> /dev/null; then
    fail "Could not connect to minikube apiserver"
fi
if ! hash curl 1>/dev/null; then
    fail "curl needs to be installed."
fi
if ! hash docker 1>/dev/null; then
    fail "Docker needs to be installed."
fi
if ! docker version 1> /dev/null; then
    fail "Could not connect to Docker daemon"
fi
if ! hash helm 1>/dev/null; then
    fail "Helm needs to be installed."
fi
