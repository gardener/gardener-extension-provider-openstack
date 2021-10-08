// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seedadmissioncontroller

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener-resource-manager/pkg/controller/garbagecollector/references"
	admissionv1 "k8s.io/api/admission/v1"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission/extensioncrds"
	"github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission/extensionresources"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// Name is used as metadata.name of the ServiceAccount, ManagedResource,
	// ClusterRole, ClusterRoleBinding, Service, Deployment and ValidatingWebhookConfiguration
	// of the seed admission controller.
	Name = "gardener-seed-admission-controller"

	managedResourceName = Name
	deploymentName      = Name
	containerName       = Name

	port            = 10250
	volumeName      = Name + "-tls"
	volumeMountPath = "/srv/gardener-seed-admission-controller"

	defaultReplicas = int32(3)
)

// New creates a new instance of DeployWaiter for the gardener-seed-admission-controller.
func New(c client.Client, namespace string, image string) component.DeployWaiter {
	return &gardenerSeedAdmissionController{
		client:    c,
		namespace: namespace,
		image:     image,
	}
}

type gardenerSeedAdmissionController struct {
	client    client.Client
	namespace string
	image     string
}

func (g *gardenerSeedAdmissionController) Deploy(ctx context.Context) error {
	replicas, err := g.getReplicas(ctx)
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Name + "-tls",
			Namespace: g.namespace,
			Labels:    getLabels(),
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSCertKey:       []byte(tlsServerCert),
			corev1.TLSPrivateKeyKey: []byte(tlsServerKey),
		},
	}
	utilruntime.Must(kutil.MakeUnique(secret))

	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      Name,
				Namespace: g.namespace,
				Labels:    getLabels(),
			},
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   Name,
				Labels: getLabels(),
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{apiextensionsv1.SchemeGroupVersion.Group},
					Resources: []string{"customresourcedefinitions"},
					Verbs:     []string{"get", "list"},
				},
				{
					APIGroups: []string{extensionsv1alpha1.SchemeGroupVersion.Group},
					Resources: []string{
						"backupbuckets",
						"backupentries",
						"bastions",
						"containerruntimes",
						"controlplanes",
						"extensions",
						"infrastructures",
						"networks",
						"operatingsystemconfigs",
						"workers",
						"clusters",
					},
					Verbs: []string{"get", "list"},
				},
			},
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   Name,
				Labels: getLabels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRole.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			}},
		}

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      Name,
				Namespace: g.namespace,
				Labels:    getLabels(),
			},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeClusterIP,
				Selector: getLabels(),
				Ports: []corev1.ServicePort{{
					Name:       "web",
					Port:       443,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(port),
				}},
			},
		}

		// if maxUnavailable would not be set, new pods don't come up in small seed clusters
		// (due to the pod anti affinity new pods are stuck in pending state)
		maxUnavailable = intstr.FromInt(1)
		deployment     = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: g.namespace,
				Labels:    getLabels(),
			},
			Spec: appsv1.DeploymentSpec{
				RevisionHistoryLimit: pointer.Int32(1),
				Replicas:             &replicas,
				Strategy: appsv1.DeploymentStrategy{
					Type: appsv1.RollingUpdateDeploymentStrategyType,
					RollingUpdate: &appsv1.RollingUpdateDeployment{
						MaxUnavailable: &maxUnavailable,
					},
				},
				Selector: &metav1.LabelSelector{MatchLabels: getLabels()},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: getLabels()},
					Spec: corev1.PodSpec{
						Affinity: &corev1.Affinity{
							PodAntiAffinity: &corev1.PodAntiAffinity{
								PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
									{
										Weight: 100,
										PodAffinityTerm: corev1.PodAffinityTerm{
											TopologyKey:   corev1.LabelHostname,
											LabelSelector: &metav1.LabelSelector{MatchLabels: getLabels()},
										},
									},
								},
							},
						},
						ServiceAccountName: serviceAccount.Name,
						Containers: []corev1.Container{{
							Name:            containerName,
							Image:           g.image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"/gardener-seed-admission-controller",
								fmt.Sprintf("--port=%d", port),
								fmt.Sprintf("--tls-cert-dir=%s", volumeMountPath),
								fmt.Sprintf("--allow-invalid-extension-resources=%t", !gardenletfeatures.FeatureGate.Enabled(features.DenyInvalidExtensionResources)),
							},
							Ports: []corev1.ContainerPort{{
								ContainerPort: int32(port),
							}},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("20m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("100Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      volumeName,
								MountPath: volumeMountPath,
								ReadOnly:  true,
							}},
						}},
						Volumes: []corev1.Volume{{
							Name: volumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secret.Name,
								},
							},
						}},
					},
				},
			},
		}

		podDisruptionBudget = &policyv1beta1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      Name,
				Namespace: g.namespace,
				Labels:    getLabels(),
			},
			Spec: policyv1beta1.PodDisruptionBudgetSpec{
				MaxUnavailable: &maxUnavailable,
				Selector: &metav1.LabelSelector{
					MatchLabels: getLabels(),
				},
			},
		}

		updateMode = autoscalingv1beta2.UpdateModeAuto
		vpa        = &autoscalingv1beta2.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      Name + "-vpa",
				Namespace: g.namespace,
				Labels:    getLabels(),
			},
			Spec: autoscalingv1beta2.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       deployment.Name,
				},
				UpdatePolicy: &autoscalingv1beta2.PodUpdatePolicy{
					UpdateMode: &updateMode,
				},
			},
		}

		caBundle                       = []byte(TLSCACert)
		validatingWebhookConfiguration = GetValidatingWebhookConfig(caBundle, service)
	)

	utilruntime.Must(references.InjectAnnotations(deployment))

	resources, err := registry.AddAllAndSerialize(
		serviceAccount,
		clusterRole,
		clusterRoleBinding,
		service,
		secret,
		deployment,
		podDisruptionBudget,
		vpa,
		validatingWebhookConfiguration,
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, g.client, g.namespace, managedResourceName, false, resources)
}

func (g *gardenerSeedAdmissionController) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, g.client, g.namespace, managedResourceName)
}

// GetValidatingWebhookConfig returns the ValidatingWebhookConfiguration for the seedadmissioncontroller component for
// reuse between the component and integration tests.
func GetValidatingWebhookConfig(caBundle []byte, webhookClientService *corev1.Service) *admissionregistrationv1.ValidatingWebhookConfiguration {
	var (
		failurePolicy = admissionregistrationv1.Fail
		matchPolicy   = admissionregistrationv1.Exact
		sideEffect    = admissionregistrationv1.SideEffectClassNone
	)
	return &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   Name,
			Labels: getLabels(),
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{{
			Name: "crds.seed.admission.core.gardener.cloud",
			Rules: []admissionregistrationv1.RuleWithOperations{{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{apiextensionsv1.GroupName},
					APIVersions: []string{apiextensionsv1beta1.SchemeGroupVersion.Version, apiextensionsv1.SchemeGroupVersion.Version},
					Resources:   []string{"customresourcedefinitions"},
				},
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Delete},
			}},
			FailurePolicy:     &failurePolicy,
			NamespaceSelector: &metav1.LabelSelector{},
			ObjectSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{gutil.DeletionProtected: "true"},
			},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				CABundle: caBundle,
				Service: &admissionregistrationv1.ServiceReference{
					Name:      webhookClientService.Name,
					Namespace: webhookClientService.Namespace,
					Path:      pointer.String(extensioncrds.WebhookPath),
				},
			},
			AdmissionReviewVersions: []string{admissionv1beta1.SchemeGroupVersion.Version, admissionv1.SchemeGroupVersion.Version},
			MatchPolicy:             &matchPolicy,
			SideEffects:             &sideEffect,
			TimeoutSeconds:          pointer.Int32(10),
		}, {
			Name: "crs.seed.admission.core.gardener.cloud",
			Rules: []admissionregistrationv1.RuleWithOperations{{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{extensionsv1alpha1.SchemeGroupVersion.Group},
					APIVersions: []string{extensionsv1alpha1.SchemeGroupVersion.Version},
					Resources: []string{
						"backupbuckets",
						"backupentries",
						"bastions",
						"containerruntimes",
						"controlplanes",
						"dnsrecords",
						"extensions",
						"infrastructures",
						"networks",
						"operatingsystemconfigs",
						"workers",
					},
				},
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Delete},
			}},
			FailurePolicy:     &failurePolicy,
			NamespaceSelector: &metav1.LabelSelector{},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				CABundle: caBundle,
				Service: &admissionregistrationv1.ServiceReference{
					Name:      webhookClientService.Name,
					Namespace: webhookClientService.Namespace,
					Path:      pointer.String(extensioncrds.WebhookPath),
				},
			},
			AdmissionReviewVersions: []string{admissionv1beta1.SchemeGroupVersion.Version, admissionv1.SchemeGroupVersion.Version},
			MatchPolicy:             &matchPolicy,
			SideEffects:             &sideEffect,
			TimeoutSeconds:          pointer.Int32(10),
		}, {
			Name: "validation.extensions.seed.admission.core.gardener.cloud",
			Rules: []admissionregistrationv1.RuleWithOperations{{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{extensionsv1alpha1.SchemeGroupVersion.Group},
					APIVersions: []string{extensionsv1alpha1.SchemeGroupVersion.Version},
					Resources: []string{
						"backupbuckets",
						"backupentries",
						"bastions",
						"containerruntimes",
						"controlplanes",
						"dnsrecords",
						"extensions",
						"infrastructures",
						"networks",
						"operatingsystemconfigs",
						"workers",
					},
				},
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
			}},
			FailurePolicy:     &failurePolicy,
			NamespaceSelector: &metav1.LabelSelector{},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				CABundle: caBundle,
				Service: &admissionregistrationv1.ServiceReference{
					Name:      webhookClientService.Name,
					Namespace: webhookClientService.Namespace,
					Path:      pointer.String(extensionresources.WebhookPath),
				},
			},
			AdmissionReviewVersions: []string{admissionv1beta1.SchemeGroupVersion.Version, admissionv1.SchemeGroupVersion.Version},
			MatchPolicy:             &matchPolicy,
			SideEffects:             &sideEffect,
			TimeoutSeconds:          pointer.Int32(10),
		}},
	}
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  "gardener",
		v1beta1constants.LabelRole: "seed-admission-controller",
	}
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (g *gardenerSeedAdmissionController) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, g.client, g.namespace, managedResourceName)
}

func (g *gardenerSeedAdmissionController) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, g.client, g.namespace, managedResourceName)
}

func (g *gardenerSeedAdmissionController) getReplicas(ctx context.Context) (int32, error) {
	nodeList := &metav1.PartialObjectMetadataList{}
	nodeList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("NodeList"))

	err := g.client.List(ctx, nodeList, client.Limit(defaultReplicas))
	if err != nil {
		return 0, err
	}

	nodeCount := int32(len(nodeList.Items))
	if nodeCount < defaultReplicas {
		return nodeCount, nil
	}

	return defaultReplicas, nil
}

const (
	// TLSCACert is the of certificate authority of the
	// seed admission controller server.
	// TODO(mvladev) this cert is hard-coded.
	// fix it in another PR.
	TLSCACert = `-----BEGIN CERTIFICATE-----
MIIC+jCCAeKgAwIBAgIUTp3XvhrWOVM8ZGe86YoXMV/UJ7AwDQYJKoZIhvcNAQEL
BQAwFTETMBEGA1UEAxMKa3ViZXJuZXRlczAeFw0xOTAyMjcxNTM0MDBaFw0yNDAy
MjYxNTM0MDBaMBUxEzARBgNVBAMTCmt1YmVybmV0ZXMwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQCyi0QGOcv2bTf3N8OLN97RwsgH6QAr8wSpAOrttBJg
FnfnU2T1RHgxm7qd190WL8DChv0dZf76d6eSQ4ZrjjyArTzufb4DtPwg+VWq7XvF
BNyn+2hf4SySkwd6k7XLhUTRx048IbByC4v+FEvmoLAwrc0d0G14ec6snD+7jO7e
kykQ/NgAOL7P6kDs9z6+bOfgF0nGN+bmeWQqJejR0t+OyQDCx5/FMtUfEVR5QX80
aeefgp3JFZb6fAw9KhLtdRV3FP0tz6hS+e4Sg0mwAAOqijZsV87kP5GYzjtcfA12
lDYl/nb1GtVvvkQD49VnV7mDnl6mG3LCMNCNH6WlZNv3AgMBAAGjQjBAMA4GA1Ud
DwEB/wQEAwIBBjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBSFA3LvJM21d8qs
ZVVCe6RrTT9wiTANBgkqhkiG9w0BAQsFAAOCAQEAns/EJ3yKsjtISoteQ714r2Um
BMPyUYTTdRHD8LZMd3RykvsacF2l2y88Nz6wJcAuoUj1h8aBDP5oXVtNmFT9jybS
TXrRvWi+aeYdb556nEA5/a94e+cb+Ck3qy/1xgQoS457AZQODisDZNBYWkAFs2Lc
ucpcAtXJtIthVm7FjoAHYcsrY04yAiYEJLD02TjUDXg4iGOGMkVHdmhawBDBF3Aj
esfcqFwji6JyAKFRACPowykQONFwUSom89uYESSCJFvNCk9MJmjJ2PzDUt6CypR4
epFdd1fXLwuwn7fvPMmJqD3HtLalX1AZmPk+BI8ezfAiVcVqnTJQMXlYPpYe9A==
-----END CERTIFICATE-----`
	tlsServerCert = `-----BEGIN CERTIFICATE-----
MIID0zCCArugAwIBAgIUaDMrqx0VRoOmGHM1afdZt39e2tMwDQYJKoZIhvcNAQEL
BQAwFTETMBEGA1UEAxMKa3ViZXJuZXRlczAeFw0yMDAzMTYxODE4MDBaFw0zMDAz
MTQxODE4MDBaMC0xKzApBgNVBAMTImdhcmRlbmVyLXNlZWQtYWRtaXNzaW9uLWNv
bnRyb2xsZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDFqFORLK0P
+h2JxhyqCK850yviF0fByRqffpHfaRyfkGt33VrFXeuhGL+suTicfhzZSWMVojk/
9R3R8FkK02Emq544o9YY5Ho/FGwlE9s1l456dW4F7oblvw7dgcRFdO6N4h/xrVab
5qdNORnxRIZTJ3qz1ZjgcsOwjyzJwyO9PidlG6MW0qqX9Ab+g8Px0eSP2zBhqcLV
6uGy4gYc2+RiXfKgYCsOu+HuNb4DFVediM82J0ZYzchMe5Uqp+PYiBIAH0Xqqz36
GW9rb5O43V5R1HSVDioFrI0EkzWYLFGxol+4TRTNA4sjPXjAJFSXDr6gy6mNYqeI
6DbThhDPMPwtAgMBAAGjggEBMIH+MA4GA1UdDwEB/wQEAwIFoDATBgNVHSUEDDAK
BggrBgEFBQcDATAMBgNVHRMBAf8EAjAAMB0GA1UdDgQWBBQf/8Y23xQcoH8EYnWh
yLGZ31Bk4DAfBgNVHSMEGDAWgBSFA3LvJM21d8qsZVVCe6RrTT9wiTCBiAYDVR0R
BIGAMH6CImdhcmRlbmVyLXNlZWQtYWRtaXNzaW9uLWNvbnRyb2xsZXKCKWdhcmRl
bmVyLXNlZWQtYWRtaXNzaW9uLWNvbnRyb2xsZXIuZ2FyZGVugi1nYXJkZW5lci1z
ZWVkLWFkbWlzc2lvbi1jb250cm9sbGVyLmdhcmRlbi5zdmMwDQYJKoZIhvcNAQEL
BQADggEBALEsnx+Zcv3IME/Xs82x0PAxDuIFV4ZnGPbweCZ5JKKlAtHtrq2JTYoQ
zHbGTj2IEpzdq04RyRqY0ejD25HWeVHcAlhSLGvKKuuMznIl6e4G/Kfmg0NLwiMK
7jsSjpNdHnJOsPg3j3iblP0ZSY8A5p12uqMzfvKPNFK62EuyqmEfI9ec6P6wNAcZ
R3Ejum8yCcOCZlOczOH/8ZIdIC1jlFYm4Wwzm1uUgoSk240nqhuBirWqARjJNhfu
/0HDmy6Zs/2FlRNIWuskpNIgOtMa3A277qx2O542+UhKv2jaIXtX1BnRLTCFVyDZ
gj5593AJYDj8QFHulFeMeh5baOkksjc=
-----END CERTIFICATE-----`
	tlsServerKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAxahTkSytD/odicYcqgivOdMr4hdHwckan36R32kcn5Brd91a
xV3roRi/rLk4nH4c2UljFaI5P/Ud0fBZCtNhJqueOKPWGOR6PxRsJRPbNZeOenVu
Be6G5b8O3YHERXTujeIf8a1Wm+anTTkZ8USGUyd6s9WY4HLDsI8sycMjvT4nZRuj
FtKql/QG/oPD8dHkj9swYanC1erhsuIGHNvkYl3yoGArDrvh7jW+AxVXnYjPNidG
WM3ITHuVKqfj2IgSAB9F6qs9+hlva2+TuN1eUdR0lQ4qBayNBJM1mCxRsaJfuE0U
zQOLIz14wCRUlw6+oMupjWKniOg204YQzzD8LQIDAQABAoIBAFfUvp2qHpUU7X9F
W4NrLIIjhkKHWcmQ1ZW+JpACI0f8YuT2pdlCLOx/FN1pyPAxUhxz8eWxGoODJmcd
yFN5LpiCdmJw2zhgfrn9Fzk6o5Qi7psYB3X3UlZRGgfwHAlJNqAxtUQtZGkOi5VT
JGYDrzTQPEQhTDegh7izRpG5du4mIXqkrmzTWIwPznLRmAps0fJQuQ9WIUP0iJSt
CMLZ0898GANcdDbE8Ta3emPe3cgJjdUTyH3zMsnJT014N0zzX+e5aXcfxCwAaN+T
fGLaQe1PV714SIhuDo+KBSJo0K0poUA8d5lNIeetl8WD0cpAKjBzpf3CvF78cT3i
c/ZrxIECgYEAzBnDosYxuKIk8iVTe+eTRRwZsi7svaMTnmlcc6/3q5vLb3i1z5V9
n/CEP5ZlvikhNB/Dt3WXmgprHzQN2ljnIJn2KHkR0gWe57aCbYtGxOCijvsZGUoJ
F2iOLfTHBsnxiNP3uzjsuceCuiSD87e0bVBJon4oz6Y7eF6kzKRGFn0CgYEA9+sj
UYtjGZfsEYChtTObC0SLXawkzAGUgJUN1NAh2w5o9Gr61Itt+SwwhqQFQhXyW50d
+bsck3Jk6U6Hke+h9ITUB3Hnqg3KW9L8sPYPqCBT6EQ/qZPmWOKjZTyiSjO1kKx6
+aPM4NKZttzJOcVwQU9m19dvM3xqUfXFPkCve3ECgYAHFcHfzad+NEq6CServm8z
T/VoZQ6cyqNstVWbQnmDgIYAWZ1eFl9lBPFiT7M6da0MZSnjHXbkxwXO8Hymnr1v
OUj9QK6orr9EZeaDLPmI7g9WjUriwNot8Ng2qi+agbobuNf5rNEy5cUY9xmJhVAD
F21m8aAzDR81X3uzCuTP9QKBgQCu9zfZ2PF7oohsYce+Rklpzlo9JbxibcsMZCV6
x9jc7HKN7OJRFoXqkJE+tIsxdKOynFQHZ1JnjRhCv7VV/TTjiMrK5kyE626hF2pW
yZGLKiWNin0ThNnQaUK/s+clTxEYpWG0xTFWicsKDw/Ewd7TeOIv+k70mx298iHe
KXCvQQKBgGdI3bZ1xxKMWDeU5QamDaOkHeZl2SacEQ+C09/O975HLfE05+gsPYDE
+YNg06oQlO/U9tmOvyGX+Ca6yLF/XQMq62oNlp1a0oqnWlQnv57rgKrGXcv2+6sP
LhAfbwDR/NNiimZioPeJEGPocUq21OL5RFjj2Sz5l4NYk6Mmeyfz
-----END RSA PRIVATE KEY-----`
)
