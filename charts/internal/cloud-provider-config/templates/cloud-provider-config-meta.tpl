{{- define "cloud-provider-config-meta"}}
{{- if and (semverCompare ">= 1.10.1" .Values.kubernetesVersion) (semverCompare "< 1.10.3" .Values.kubernetesVersion) }}
[Metadata]
{{- if (ne .Values.dhcpDomain "") }}
dhcp-domain="{{ .Values.dhcpDomain }}"
{{- end }}
{{- if (ne .Values.requestTimeout "") }}
request-timeout={{ .Values.requestTimeout }}
{{- end }}
{{- end }}
{{- end }}