{{- define "echo" -}}
nested-{{ .Echo | trim }}
{{- end }}