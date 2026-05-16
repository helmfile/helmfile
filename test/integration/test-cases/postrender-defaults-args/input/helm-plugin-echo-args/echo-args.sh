#!/usr/bin/env bash

set -e

arg=$2
input=$(cat)

echo "$input"

echo "---"

cat <<EOS
apiVersion: v1
kind: ConfigMap
metadata:
  name: rendered-arg-${arg}
data:
  arg: ${arg}
EOS
