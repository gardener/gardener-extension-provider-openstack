{{- define "csi-driver-node.extensionsGroup" -}}
extensions.gardener.cloud
{{- end -}}

{{- define "csi-driver-node.name" -}}
provider-openstack
{{- end -}}

{{- define "csi-driver-node.provisioner" -}}
cinder.csi.openstack.org
{{- end -}}

{{- define "csi-driver-node.storageversion" -}}
storage.k8s.io/v1
{{- end -}}