#!/usr/bin/env bash
cat
echo "---"
cat <<EOS
apiVersion: v1
kind: ConfigMap
data:
  two: TWO
metadata:
  name: cm2
EOS
