#!/usr/bin/env bash
configmap_name=$1
cat
echo "---"

cat <<EOS
apiVersion: v1
kind: ConfigMap
data:
  name: ${configmap_name} 
metadata:
  name: ${configmap_name} 
EOS
