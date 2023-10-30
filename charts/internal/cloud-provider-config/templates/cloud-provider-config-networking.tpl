{{- define "cloud-provider-config-networking" -}}
[Networking]
{{- if .Values.internalNetworkName }}
internal-network-name="{{ .Values.internalNetworkName }}"
{{- end }}
{{- end -}}
