#!/usr/bin/env bash
arg=$1
cat
echo "---"
cat <<EOS
apiVersion: v1
kind: ConfigMap
metadata:
  name: rendered-arg-${arg}
data:
  arg: ${arg}
EOS
