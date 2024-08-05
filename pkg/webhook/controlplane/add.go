// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	oscutils "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/utils"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

var (
	logger = log.Log.WithName("openstack-controlplane-webhook")
	// TODO(LucaBernstein): Clean up the gardener version check after October/2024.
	versionConstraintGreaterEqual198 *semver.Constraints
)

func init() {
	var err error
	versionConstraintGreaterEqual198, err = semver.NewConstraint(">= 1.98")
	utilruntime.Must(err)
}

// AddToManager creates a webhook and adds it to the manager.
func AddToManager(gardenerVersion *string) func(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	return func(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
		var objectSelector *metav1.LabelSelector
		if gardenerVersion != nil && len(*gardenerVersion) > 0 {
			version, err := semver.NewVersion(*gardenerVersion)
			if err != nil {
				return nil, fmt.Errorf("failed to parse gardener version: %v", err)
			}
			if versionConstraintGreaterEqual198.Check(version) {
				objectSelector = &metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelExtensionProviderMutatedByControlplaneWebhook: "true"}}
			}
		}

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
			ObjectSelector: objectSelector,
			Mutator: genericmutator.NewMutator(mgr, NewEnsurer(logger), oscutils.NewUnitSerializer(),
				kubelet.NewConfigCodec(fciCodec), fciCodec, logger),
		})
	}
}
