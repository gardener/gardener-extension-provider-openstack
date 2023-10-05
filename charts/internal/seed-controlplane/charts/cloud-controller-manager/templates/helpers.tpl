{{- define "cloud-controller-manager.featureGates" -}}
{{- if .Values.featureGates }}
- --feature-gates={{ range $feature, $enabled := .Values.featureGates }}{{ $feature }}={{ $enabled }},{{ end }}
{{- end }}
{{- end -}}

{{- define "cloud-controller-manager.port" -}}
10258
{{- end -}}

{{- define "deploymentversion" -}}
apps/v1
{{- end -}}
