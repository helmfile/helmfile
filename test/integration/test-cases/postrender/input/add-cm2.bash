#!/usr/bin/env bash

echo $1
cat $1
echo "---"

cat <<EOS
apiVersion: v1
kind: ConfigMap
data:
  two: TWO
metadata:
  name: cm2
EOS
