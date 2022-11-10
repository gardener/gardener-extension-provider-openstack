{{- define "cloud-provider-config-route" -}}
[Route]
{{- if .Values.routerID }}
router-id="{{ .Values.routerID }}"
{{- end }}
{{- end -}}
