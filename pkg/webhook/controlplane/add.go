// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	oscutils "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/utils"
	appsv1 "k8s.io/api/apps/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

var (
	logger = log.Log.WithName("openstack-controlplane-webhook")
)

// AddToManager creates a webhook and adds it to the manager.
func AddToManager(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	logger.Info("Adding webhook to manager")
	fciCodec := oscutils.NewFileContentInlineCodec()
	return controlplane.New(mgr, controlplane.Args{
		Kind:     controlplane.KindShoot,
		Provider: openstack.Type,
		Types: []extensionswebhook.Type{
			{Obj: &appsv1.Deployment{}},
			{Obj: &vpaautoscalingv1.VerticalPodAutoscaler{}},
			{Obj: &extensionsv1alpha1.OperatingSystemConfig{}},
		},
		Mutator: genericmutator.NewMutator(mgr, NewEnsurer(logger), oscutils.NewUnitSerializer(),
			kubelet.NewConfigCodec(fciCodec), fciCodec, logger),
	})
}
