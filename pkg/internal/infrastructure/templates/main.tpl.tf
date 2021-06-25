provider "openstack" {
  auth_url    = "{{ .openstack.authURL }}"
  domain_name = var.DOMAIN_NAME
  tenant_name = var.TENANT_NAME
  region      = "{{ .openstack.region }}"
  user_name   = var.USER_NAME
  password    = var.PASSWORD
  application_credential_id     = var.APPLICATION_CREDENTIAL_ID
  application_credential_secret = var.APPLICATION_CREDENTIAL_SECRET
  insecure    = true
  max_retries = "{{ .openstack.maxApiCallRetries }}"
}

//=====================================================================
//= Networking: Router/Interfaces/Net/SubNet/SecGroup/SecRules
//=====================================================================

data "openstack_networking_network_v2" "fip" {
  name = "{{ .openstack.floatingPoolName }}"
}

{{ if .create.router -}}
{{ if .router.floatingPoolSubnet -}}
data "openstack_networking_subnet_ids_v2" "fip_subnets" {
  name_regex = {{ .router.floatingPoolSubnet | quote }}
  network_id = data.openstack_networking_network_v2.fip.id
}
{{- end }}

resource "openstack_networking_router_v2" "router" {
  name                = "{{ .clusterName }}"
  region              = "{{ .openstack.region }}"
  external_network_id = data.openstack_networking_network_v2.fip.id
  {{ if .router.enableSNAT -}}
  enable_snat         = true
  {{- end }}
  {{ if .router.floatingPoolSubnet -}}
  external_subnet_ids = data.openstack_networking_subnet_ids_v2.fip_subnets.ids
  {{- end }}
}
{{- end}}

resource "openstack_networking_network_v2" "cluster" {
  name           = "{{ .clusterName }}"
  admin_state_up = "true"
}

resource "openstack_networking_subnet_v2" "cluster" {
  name            = "{{ .clusterName }}"
  cidr            = "{{ .networks.workers }}"
  network_id      = openstack_networking_network_v2.cluster.id
  ip_version      = 4
  {{- if .dnsServers }}
  dns_nameservers = [{{- dnsServers .dnsServers }}]
  {{- else }}
  dns_nameservers = []
  {{- end }}
}

resource "openstack_networking_router_interface_v2" "router_nodes" {
  router_id = {{ .router.id }}
  subnet_id = openstack_networking_subnet_v2.cluster.id
}

resource "openstack_networking_secgroup_v2" "cluster" {
  name                 = "{{ .clusterName }}"
  description          = "Cluster Nodes"
  delete_default_rules = true
}

resource "openstack_networking_secgroup_rule_v2" "cluster_self" {
  direction         = "ingress"
  ethertype         = "IPv4"
  security_group_id = openstack_networking_secgroup_v2.cluster.id
  remote_group_id   = openstack_networking_secgroup_v2.cluster.id
}

resource "openstack_networking_secgroup_rule_v2" "cluster_egress" {
  direction         = "egress"
  ethertype         = "IPv4"
  security_group_id = openstack_networking_secgroup_v2.cluster.id
}

resource "openstack_networking_secgroup_rule_v2" "cluster_tcp_all" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 1
  port_range_max    = 65535
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = openstack_networking_secgroup_v2.cluster.id
}

resource "openstack_networking_secgroup_rule_v2" "cluster_udp_all" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "udp"
  port_range_min    = 1
  port_range_max    = 65535
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = openstack_networking_secgroup_v2.cluster.id
}

//=====================================================================
//= SSH Key for Nodes (Bastion and Worker)
//=====================================================================

resource "openstack_compute_keypair_v2" "ssh_key" {
  name       = "{{ .clusterName }}"
  public_key = "{{ .sshPublicKey }}"
}

// We have introduced new output variables. However, they are not applied for
// existing clusters as Terraform won't detect a diff when we run `terraform plan`.
// Workaround: Providing a null-resource for letting Terraform think that there are
// differences, enabling the Gardener to start an actual `terraform apply` job.
resource "null_resource" "outputs" {
  triggers = {
    recompute = "outputs"
  }
}

//=====================================================================
//= Output Variables
//=====================================================================

output "{{ .outputKeys.routerID }}" {
  value = {{ .router.id }}
}

output "{{ .outputKeys.networkID }}" {
  value = openstack_networking_network_v2.cluster.id
}

output "{{ .outputKeys.keyName }}" {
  value = openstack_compute_keypair_v2.ssh_key.name
}

output "{{ .outputKeys.securityGroupID }}" {
  value = openstack_networking_secgroup_v2.cluster.id
}

output "{{ .outputKeys.securityGroupName }}" {
  value = openstack_networking_secgroup_v2.cluster.name
}

output "{{ .outputKeys.floatingNetworkID }}" {
  value = data.openstack_networking_network_v2.fip.id
}

output "{{ .outputKeys.subnetID }}" {
  value = openstack_networking_subnet_v2.cluster.id
}
