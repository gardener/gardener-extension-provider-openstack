{{- define "cloud-provider-config-credentials" -}}
auth-url="{{ .Values.authUrl }}"
domain-name="{{ .Values.domainName }}"
tenant-name="{{ .Values.tenantName }}"
username="{{ .Values.username }}"
{{- if .Values.password }}
password="{{ .Values.password }}"
{{- end }}
{{- if .Values.applicationCredentialSecret }}
application-credential-id="{{ .Values.applicationCredentialID }}"
application-credential-name="{{ .Values.applicationCredentialName }}"
application-credential-secret="{{ .Values.applicationCredentialSecret }}"
{{- end }}
region="{{ .Values.region }}"
{{- if .Values.insecure }}
tls-insecure={{ .Values.insecure }}
{{- end }}
{{- if .Values.caCert }}
ca-file="/etc/kubernetes/cloudprovider/keystone-ca.crt"
{{- end }}
{{- end -}}
