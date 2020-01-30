{{- define "storageclassversion" -}}
{{- if semverCompare ">= 1.13-0" .Capabilities.KubeVersion.GitVersion -}}
storage.k8s.io/v1
{{- else -}}
storage.k8s.io/v1beta1
{{- end -}}
{{- end -}}
