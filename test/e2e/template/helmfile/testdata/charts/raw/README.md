You should be able to test pushing this chart to a local registry with:

```
$ helm package .
Successfully packaged chart and saved it to: /home/mumoshu/p/helmfile/test/e2e/template/helmfile/testdata/charts/raw/raw-0.1.0.tgz

$ helm push raw-0.1.0.tgz oci://localhost:5000/myrepo/raw
Pushed: localhost:5000/myrepo/raw/raw:0.1.0
Digest: sha256:9b7c9633b519b024fdbec1db795bc2dd8b0009149135908a3aafc55280146ad9
```
