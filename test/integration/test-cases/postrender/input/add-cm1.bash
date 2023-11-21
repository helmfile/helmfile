#!/usr/bin/env bash

# cat $1
# echo "---"
cat <<EOS
apiVersion: v1
kind: ConfigMap
data:
  one: ONE
metadata:
  name: cm1
EOS
