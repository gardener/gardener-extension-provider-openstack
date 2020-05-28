{{- define "cloud-provider-config-credentials" -}}
auth-url="{{ .Values.authUrl }}"
domain-name="{{ .Values.domainName }}"
tenant-name="{{ .Values.tenantName }}"
username="{{ .Values.username }}"
password="{{ .Values.password }}"
{{- end -}}
