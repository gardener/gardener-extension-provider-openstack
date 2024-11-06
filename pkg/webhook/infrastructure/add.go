// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

const (
	// WebhookName is the used for the Infrastructure webhook.
	WebhookName = "infrastructure"
	webhookPath = "infrastructure"
)

var logger = log.Log.WithName("infrastructure-webhook")

// AddToManager creates an Infrastructure webhook adds the webhook to the manager.
func AddToManager(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	logger.Info("Adding webhook to manager")

	types := []extensionswebhook.Type{
		{Obj: &extensionsv1alpha1.Infrastructure{}},
	}

	handler, err := extensionswebhook.NewBuilder(mgr, logger).WithMutator(New(mgr, logger), types...).Build()
	if err != nil {
		return nil, err
	}

	logger.Info("Creating webhook")
	return &extensionswebhook.Webhook{
		Name:              WebhookName,
		Target:            extensionswebhook.TargetSeed,
		Provider:          openstack.Type,
		Types:             types,
		Webhook:           &admission.Webhook{Handler: handler, RecoverPanic: ptr.To(true)},
		Path:              webhookPath,
		NamespaceSelector: buildSelector(openstack.Type),
	}, nil
}

func buildSelector(provider string) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      v1beta1constants.LabelShootProvider,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{provider},
			},
		},
	}
}
