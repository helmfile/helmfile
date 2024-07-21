# Secrets

helmfile can handle secrets using [helm-secrets](https://github.com/jkroepke/helm-secrets) plugin or using remote secrets storage
(everything that package [vals](https://github.com/helmfile/vals) can handle vault, AWS SSM etc)
This section will describe the second use case.

# Remote secrets

This paragraph will describe how to use remote secrets storage (vault, SSM etc) in helmfile

## Fetching single key

To fetch single key from remote secret storage you can use `fetchSecretValue` template function example below

```yaml
# helmfile.yaml

repositories:
  - name: stable
    url: https://kubernetes-charts.storage.googleapis.com

environments:
  default:
    values:
      - service:
          password: ref+vault://svc/#pass
          login: ref+vault://svc/#login
releases:
  - name: service
    namespace: default
    labels:
      cluster: services
      secrets: vault
    chart: stable/svc
    version: 0.1.0
    values:
      - service:
          login: {{ .Values.service.login | fetchSecretValue }} # this will resolve ref+vault://svc/#pass and fetch secret from vault
          password: {{ .Values.service.password | fetchSecretValue | quote }}
      # - values/service.yaml.gotmpl   # alternatively
```
## Fetching multiple keys
Alternatively you can use `expandSecretRefs` to fetch a map of secrets
```yaml
# values/service.yaml.gotmpl
service:
{{ .Values.service | expandSecretRefs | toYaml | nindent 2 }}
```

This will produce
```yaml
# values/service.yaml
service:
  login: svc-login # fetched from vault
  password: pass

```