{{- define "cloud-provider-config-loadbalancer" -}}
[LoadBalancer]
create-monitor=true
monitor-delay=60s
monitor-timeout=30s
monitor-max-retries=5
lb-version=v2
lb-provider="{{ .Values.lbProvider }}"
floating-network-id="{{ .Values.floatingNetworkID }}"
use-octavia="{{ .Values.useOctavia }}"
{{- if .Values.floatingSubnetID }}
floating-subnet-id="{{ .Values.floatingSubnetID }}"
{{- end }}
{{- if .Values.subnetID }}
subnet-id="{{ .Values.subnetID }}"
{{- end }}
{{- include "cloud-provider-config-meta" . | indent 4 }}
{{- range $i, $class := .Values.floatingClasses }}
[LoadBalancerClass {{ $class.name | quote }}]
{{- if $class.floatingNetworkID }}
floating-network-id="{{ $class.floatingNetworkID }}"
{{- end }}
{{- if $class.floatingSubnetID }}
floating-subnet-id="{{ $class.floatingSubnetID }}"
{{- end }}
{{- if $class.subnetID }}
subnet-id="{{ $class.subnetID }}"
{{- end }}
{{- end }}
{{- end -}}
