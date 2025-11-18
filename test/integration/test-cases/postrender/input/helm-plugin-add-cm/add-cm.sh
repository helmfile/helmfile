#!/usr/bin/env bash

set -e

# Get the configmap name from the second argument (first is empty when passed via plugin)
configmap_name=$2

# For Helm 4 plugins, buffer the input
input=$(cat)

# Output the input first
echo "$input"

# Then add the separator and new ConfigMap
echo "---"

cat <<EOS
apiVersion: v1
kind: ConfigMap
data:
  name: ${configmap_name}
metadata:
  name: ${configmap_name}
EOS
