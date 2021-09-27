{{- define "cloud-provider-config-meta" -}}
[Metadata]
{{- if hasKey .Values "requestTimeout" }}
{{- if .Values.requestTimeout }}
request-timeout={{ .Values.requestTimeout }}
{{- end }}
{{- end }}
{{- end -}}
