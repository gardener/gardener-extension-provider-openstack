{{- define "cloud-provider-config-credentials" -}}
auth-url="{{ .Values.authUrl }}"
domain-name="{{ .Values.domainName }}"
tenant-name="{{ .Values.tenantName }}"
{{- if .Values.username }}
username="{{ .Values.username }}"
password="{{ .Values.password }}"
{{- end }}
{{- if .Values.applicationCredentialID }}
application-credential-id="{{ .Values.applicationCredentialID }}"
application-credential-secret="{{ .Values.applicationCredentialSecret }}"
{{- end }}
region="{{ .Values.region }}"
{{- end -}}
