#!/usr/bin/env bash

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
  two: TWO
metadata:
  name: cm2
EOS
