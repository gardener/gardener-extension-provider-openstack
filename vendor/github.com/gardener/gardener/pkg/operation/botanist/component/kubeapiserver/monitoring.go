// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubeapiserver

import (
	"bytes"
	"strings"
	"text/template"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

const (
	monitoringPrometheusJobName = "kube-apiserver"

	monitoringMetricAuthenticationAttempts                                     = "authentication_attempts"
	monitoringMetricAuthenticatedUserRequests                                  = "authenticated_user_requests"
	monitoringMetricApiserverAdmissionControllerAdmissionDurationSecondsBucket = "apiserver_admission_controller_admission_duration_seconds_bucket"
	monitoringMetricApiserverAdmissionWebhookAdmissionDurationSecondsBucket    = "apiserver_admission_webhook_admission_duration_seconds_bucket"
	monitoringMetricApiserverAdmissionStepAdmissionDurationSecondsBucket       = "apiserver_admission_step_admission_duration_seconds_bucket"
	monitoringMetricApiserverAdmissionWebhookRejectionCount                    = "apiserver_admission_webhook_rejection_count"
	monitoringMetricApiserverAuditEventTotal                                   = "apiserver_audit_event_total"
	monitoringMetricApiserverAuditErrorTotal                                   = "apiserver_audit_error_total"
	monitoringMetricApiserverAuditRequestsRejectedTotal                        = "apiserver_audit_requests_rejected_total"
	monitoringMetricApiserverLatencySeconds                                    = "apiserver_latency_seconds"
	monitoringMetricApiserverCRDWebhookConversionDurationSecondsBucket         = "apiserver_crd_webhook_conversion_duration_seconds_bucket"
	monitoringMetricApiserverCurrentInflightRequests                           = "apiserver_current_inflight_requests"
	monitoringMetricApiserverCurrentInqueueRequests                            = "apiserver_current_inqueue_requests"
	monitoringMetricApiserverResponseSizesBucket                               = "apiserver_response_sizes_bucket"
	monitoringMetricApiserverRegisteredWatchers                                = "apiserver_registered_watchers"
	monitoringMetricApiserverRequestDurationSecondsBucket                      = "apiserver_request_duration_seconds_bucket"
	monitoringMetricApiserverRequestTerminationsTotal                          = "apiserver_request_terminations_total"
	monitoringMetricApiserverRequestTotal                                      = "apiserver_request_total"
	monitoringMetricApiserverRequestCount                                      = "apiserver_request_count"
	monitoringMetricApiserverStorageTransformationDurationSecondsBucket        = "apiserver_storage_transformation_duration_seconds_bucket"
	monitoringMetricApiserverStorageTransformationOperationsTotal              = "apiserver_storage_transformation_operations_total"
	monitoringMetricApiserverInitEventsTotal                                   = "apiserver_init_events_total"
	monitoringMetricApiserverWatchEventsSizesBucket                            = "apiserver_watch_events_sizes_bucket"
	monitoringMetricApiserverWatchEventsTotal                                  = "apiserver_watch_events_total"
	monitoringMetricApiserverWatchDuration                                     = "apiserver_watch_duration"
	monitoringMetricEtcdDbTotalSizeInBytes                                     = "etcd_db_total_size_in_bytes"
	monitoringMetricEtcdObjectCounts                                           = "etcd_object_counts"
	monitoringMetricEtcdRequestDurationSecondsBucket                           = "etcd_request_duration_seconds_bucket"
	monitoringMetricGo                                                         = "go_.+"
	monitoringMetricProcessMaxFds                                              = "process_max_fds"
	monitoringMetricProcessOpenFds                                             = "process_open_fds"
	monitoringMetricWatchCacheCapacityIncreaseTotal                            = "watch_cache_capacity_increase_total"
	monitoringMetricWatchCacheCapacityDecreaseTotal                            = "watch_cache_capacity_decrease_total"
	monitoringMetricWatchCacheCapacity                                         = "watch_cache_capacity"

	// TODO: Replace below hard-coded job name of the Blackbox Exporter once its deployment has been refactored.
	monitoringAlertingRules = `groups:
- name: kube-apiserver.rules
  rules:
  - alert: ApiServerNotReachable
    expr: probe_success{job="blackbox-apiserver"} == 0
    for: 5m
    labels:
      service: ` + v1beta1constants.DeploymentNameKubeAPIServer + `
      severity: blocker
      type: seed
      visibility: all
    annotations:
      description: "API server not reachable via external endpoint: {{ $labels.instance }}."
      summary: API server not reachable (externally).
  - alert: KubeApiserverDown
    expr: absent(up{job="` + monitoringPrometheusJobName + `"} == 1)
    for: 5m
    labels:
      service: ` + v1beta1constants.DeploymentNameKubeAPIServer + `
      severity: blocker
      type: seed
      visibility: operator
    annotations:
      description: All API server replicas are down/unreachable, or all API server could not be found.
      summary: API server unreachable.
  - alert: KubeApiServerTooManyOpenFileDescriptors
    expr: 100 * ` + monitoringMetricProcessOpenFds + `{job="` + monitoringPrometheusJobName + `"} / ` + monitoringMetricProcessMaxFds + ` > 50
    for: 30m
    labels:
      service: ` + v1beta1constants.DeploymentNameKubeAPIServer + `
      severity: warning
      type: seed
      visibility: owner
    annotations:
      description: 'The API server ({{ $labels.instance }}) is using {{ $value }}% of the available file/socket descriptors.'
      summary: 'The API server has too many open file descriptors'
  - alert: KubeApiServerTooManyOpenFileDescriptors
    expr: 100 * ` + monitoringMetricProcessOpenFds + `{job="` + monitoringPrometheusJobName + `"} / ` + monitoringMetricProcessMaxFds + `{job="` + monitoringPrometheusJobName + `"} > 80
    for: 30m
    labels:
      service: ` + v1beta1constants.DeploymentNameKubeAPIServer + `
      severity: critical
      type: seed
      visibility: owner
    annotations:
      description: 'The API server ({{ $labels.instance }}) is using {{ $value }}% of the available file/socket descriptors.'
      summary: 'The API server has too many open file descriptors'
  # Some verbs excluded because they are expected to be long-lasting:
  # WATCHLIST is long-poll, CONNECT is "kubectl exec".
  - alert: KubeApiServerLatency
    expr: histogram_quantile(0.99, sum without (instance,resource) (rate(` + monitoringMetricApiserverRequestDurationSecondsBucket + `{subresource!="log",verb!~"CONNECT|WATCHLIST|WATCH|PROXY proxy"}[5m]))) > 3
    for: 30m
    labels:
      service: ` + v1beta1constants.DeploymentNameKubeAPIServer + `
      severity: warning
      type: seed
      visibility: owner
    annotations:
      description: Kube API server latency for verb {{ $labels.verb }} is high. This could be because the shoot workers and the control plane are in different regions. 99th percentile of request latency is greater than 3 seconds.
      summary: Kubernetes API server latency is high
  # TODO(wyb1): replace with better metrics in the future
  - record: shoot:` + monitoringMetricApiserverWatchDuration + `:quantile
    expr: histogram_quantile(0.2, sum(rate(` + monitoringMetricApiserverRequestDurationSecondsBucket + `{verb="WATCH",resource=~"configmaps|deployments|secrets|daemonsets|services|nodes|pods|namespaces|endpoints|statefulsets|clusterroles|roles"}[5m])) by (le,scope,resource))
    labels:
      quantile: "0.2"
  - record: shoot:` + monitoringMetricApiserverWatchDuration + `:quantile
    expr: histogram_quantile(0.5, sum(rate(` + monitoringMetricApiserverRequestDurationSecondsBucket + `{verb="WATCH",resource=~"configmaps|deployments|secrets|daemonsets|services|nodes|pods|namespaces|endpoints|statefulsets|clusterroles|roles"}[5m])) by (le,scope,resource))
    labels:
      quantile: "0.5"
  - record: shoot:` + monitoringMetricApiserverWatchDuration + `:quantile
    expr: histogram_quantile(0.9, sum(rate(` + monitoringMetricApiserverRequestDurationSecondsBucket + `{verb="WATCH",resource=~"configmaps|deployments|secrets|daemonsets|services|nodes|pods|namespaces|endpoints|statefulsets|clusterroles|roles"}[5m])) by (le,scope,resource))
    labels:
      quantile: "0.9"
  - record: shoot:` + monitoringMetricApiserverWatchDuration + `:quantile
    expr: histogram_quantile(0.2, sum(rate(` + monitoringMetricApiserverRequestDurationSecondsBucket + `{verb="WATCH",group=~".+garden.+"}[5m])) by (le,scope,resource))
    labels:
      quantile: "0.2"
  - record: shoot:` + monitoringMetricApiserverWatchDuration + `:quantile
    expr: histogram_quantile(0.5, sum(rate(` + monitoringMetricApiserverRequestDurationSecondsBucket + `{verb="WATCH",group=~".+garden.+"}[5m])) by (le,scope,resource))
    labels:
      quantile: "0.5"
  - record: shoot:` + monitoringMetricApiserverWatchDuration + `:quantile
    expr: histogram_quantile(0.9, sum(rate(` + monitoringMetricApiserverRequestDurationSecondsBucket + `{verb="WATCH",group=~".+garden.+"}[5m])) by (le,scope,resource))
    labels:
      quantile: "0.9"
  ### API auditlog ###
  - alert: KubeApiServerTooManyAuditlogFailures
    expr: sum(rate (` + monitoringMetricApiserverAuditErrorTotal + `{plugin="webhook",job="` + monitoringPrometheusJobName + `"}[5m])) / sum(rate(` + monitoringMetricApiserverAuditEventTotal + `{job="` + monitoringPrometheusJobName + `"}[5m])) > bool 0.02 == 1
    for: 15m
    labels:
      service: auditlog
      severity: warning
      type: seed
      visibility: operator
    annotations:
      description: 'The API servers cumulative failure rate in logging audit events is greater than 2%.'
      summary: 'The kubernetes API server has too many failed attempts to log audit events'
  ### API latency ###
  - record: ` + monitoringMetricApiserverLatencySeconds + `:quantile
    expr: histogram_quantile(0.99, rate(` + monitoringMetricApiserverRequestDurationSecondsBucket + `[5m]))
    labels:
      quantile: "0.99"
  - record: apiserver_latency:quantile
    expr: histogram_quantile(0.9, rate(` + monitoringMetricApiserverRequestDurationSecondsBucket + `[5m]))
    labels:
      quantile: "0.9"
  - record: ` + monitoringMetricApiserverLatencySeconds + `:quantile
    expr: histogram_quantile(0.5, rate(` + monitoringMetricApiserverRequestDurationSecondsBucket + `[5m]))
    labels:
      quantile: "0.5"

  - record: shoot:kube_apiserver:sum_by_pod
    expr: sum(up{job="` + monitoringPrometheusJobName + `"}) by (pod)
`
)

var (
	monitoringAllowedMetrics = []string{
		monitoringMetricAuthenticationAttempts,
		monitoringMetricAuthenticatedUserRequests,
		monitoringMetricApiserverAdmissionControllerAdmissionDurationSecondsBucket,
		monitoringMetricApiserverAdmissionWebhookAdmissionDurationSecondsBucket,
		monitoringMetricApiserverAdmissionStepAdmissionDurationSecondsBucket,
		monitoringMetricApiserverAdmissionWebhookRejectionCount,
		monitoringMetricApiserverAuditEventTotal,
		monitoringMetricApiserverAuditErrorTotal,
		monitoringMetricApiserverAuditRequestsRejectedTotal,
		monitoringMetricApiserverLatencySeconds,
		monitoringMetricApiserverCRDWebhookConversionDurationSecondsBucket,
		monitoringMetricApiserverCurrentInflightRequests,
		monitoringMetricApiserverCurrentInqueueRequests,
		monitoringMetricApiserverResponseSizesBucket,
		monitoringMetricApiserverRegisteredWatchers,
		monitoringMetricApiserverRequestDurationSecondsBucket,
		monitoringMetricApiserverRequestTerminationsTotal,
		monitoringMetricApiserverRequestTotal,
		monitoringMetricApiserverRequestCount,
		monitoringMetricApiserverStorageTransformationDurationSecondsBucket,
		monitoringMetricApiserverStorageTransformationOperationsTotal,
		monitoringMetricApiserverInitEventsTotal,
		monitoringMetricApiserverWatchEventsSizesBucket,
		monitoringMetricApiserverWatchEventsTotal,
		monitoringMetricEtcdDbTotalSizeInBytes,
		monitoringMetricEtcdObjectCounts,
		monitoringMetricEtcdRequestDurationSecondsBucket,
		monitoringMetricGo,
		monitoringMetricProcessMaxFds,
		monitoringMetricProcessOpenFds,
		monitoringMetricWatchCacheCapacityIncreaseTotal,
		monitoringMetricWatchCacheCapacityDecreaseTotal,
		monitoringMetricWatchCacheCapacity,
	}

	// TODO: Replace below hard-coded paths to Prometheus certificates once its deployment has been refactored.
	monitoringScrapeConfigTmpl = `job_name: ` + monitoringPrometheusJobName + `
scheme: https
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: [{{ .namespace }}]
tls_config:
  insecure_skip_verify: true
  cert_file: /etc/prometheus/seed/prometheus.crt
  key_file: /etc/prometheus/seed/prometheus.key
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: ` + v1beta1constants.DeploymentNameKubeAPIServer + `;` + ServicePortName + `
- action: labelmap
  regex: __meta_kubernetes_service_label_(.+)
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(` + strings.Join(monitoringAllowedMetrics, "|") + `)$
`
	monitoringScrapeConfigTemplate *template.Template
)

func init() {
	var err error

	monitoringScrapeConfigTemplate, err = template.New("monitoring-scrape-config").Parse(monitoringScrapeConfigTmpl)
	utilruntime.Must(err)
}

// ScrapeConfigs returns the scrape configurations for Prometheus.
func (k *kubeAPIServer) ScrapeConfigs() ([]string, error) {
	var scrapeConfig bytes.Buffer

	if err := monitoringScrapeConfigTemplate.Execute(&scrapeConfig, map[string]interface{}{"namespace": k.namespace}); err != nil {
		return nil, err
	}

	return []string{scrapeConfig.String()}, nil
}

// AlertingRules returns the alerting rules for AlertManager.
func (k *kubeAPIServer) AlertingRules() (map[string]string, error) {
	return map[string]string{"kube-apiserver.rules.yaml": monitoringAlertingRules}, nil
}
