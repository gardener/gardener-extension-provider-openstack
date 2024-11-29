{{- define "name" -}}
gardener-extension-admission-openstack
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
  {{- if hasPrefix "sha256:" .tag }}
  {{- printf "%s@%s" .repository .tag }}
  {{- else }}
  {{- printf "%s:%s" .repository .tag }}
  {{- end }}
{{- end }}

{{- define "leaderelectionid" -}}
gardener-extension-admission-openstack
{{- end -}}
