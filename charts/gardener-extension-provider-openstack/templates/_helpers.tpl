{{- define "name" -}}
gardener-extension-provider-openstack
{{- end -}}

{{- define "labels.app.key" -}}
app.kubernetes.io/name
{{- end -}}
{{- define "labels.app.value" -}}
{{ include "name" . }}
{{- end -}}

{{- define "labels" -}}
{{ include "labels.app.key" . }}: {{ include "labels.app.value" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{-  define "image" -}}
  {{- if hasPrefix "sha256:" .Values.image.tag }}
  {{- printf "%s@%s" .Values.image.repository .Values.image.tag }}
  {{- else }}
  {{- printf "%s:%s" .Values.image.repository .Values.image.tag }}
  {{- end }}
{{- end }}

{{- define "deploymentversion" -}}
apps/v1
{{- end -}}

{{- define "seed.provider" -}}
  {{- if .Values.gardener.seed }}
{{- .Values.gardener.seed.provider }}
  {{- else -}}
""
  {{- end }}
{{- end -}}

{{- define "runtimeCluster.enabled" -}}
  {{- if .Values.gardener.runtimeCluster }}
{{- .Values.gardener.runtimeCluster.enabled }}
  {{- else -}}
false
  {{- end }}
{{- end -}}
