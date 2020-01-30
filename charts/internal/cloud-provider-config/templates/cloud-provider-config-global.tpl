{{- define "cloud-provider-config-global"}}
[Global]
auth-url="{{ .Values.authUrl }}"
domain-name="{{ .Values.domainName }}"
tenant-name="{{ .Values.tenantName }}"
username="{{ .Values.username }}"
password="{{ .Values.password }}"
[LoadBalancer]
create-monitor=true
monitor-delay=60s
monitor-timeout=30s
monitor-max-retries=5
lb-version=v2
lb-provider="{{ .Values.lbProvider }}"
floating-network-id="{{ .Values.floatingNetworkID }}"
{{- end }}