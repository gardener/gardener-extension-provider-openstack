// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
)

func (w *WorkerDelegate) decodeWorkerProviderStatus() (*api.WorkerStatus, error) {
	workerStatus := &api.WorkerStatus{}

	if w.worker.Status.ProviderStatus == nil {
		return workerStatus, nil
	}

	marshalled, err := w.worker.Status.GetProviderStatus().MarshalJSON()
	if err != nil {
		return nil, err
	}
	if _, _, err := w.decoder.Decode(marshalled, nil, workerStatus); err != nil {
		return nil, fmt.Errorf("could not decode WorkerStatus %q: %w", k8sclient.ObjectKeyFromObject(w.worker), err)
	}

	return workerStatus, nil
}

func (w *WorkerDelegate) updateWorkerProviderStatus(ctx context.Context, workerStatus *api.WorkerStatus) error {
	var workerStatusV1alpha1 = &v1alpha1.WorkerStatus{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "WorkerStatus",
		},
	}

	if err := w.scheme.Convert(workerStatus, workerStatusV1alpha1, nil); err != nil {
		return err
	}

	patch := k8sclient.MergeFrom(w.worker.DeepCopy())
	w.worker.Status.ProviderStatus = &runtime.RawExtension{Object: workerStatusV1alpha1}
	return w.seedClient.Status().Patch(ctx, w.worker, patch)
}

func (w *WorkerDelegate) updateMachineDependenciesStatus(ctx context.Context, workerStatus *api.WorkerStatus, serverGroupDependencies []api.ServerGroupDependency, err error) error {
	workerStatus.ServerGroupDependencies = serverGroupDependencies
	if statusUpdateErr := w.updateWorkerProviderStatus(ctx, workerStatus); statusUpdateErr != nil {
		if err != nil {
			err = fmt.Errorf("%s: %w", err.Error(), statusUpdateErr)
		} else {
			err = statusUpdateErr
		}
	}

	return err
}

// ClusterTechnicalName returns the technical name of the cluster this worker belongs.
func (w *WorkerDelegate) ClusterTechnicalName() string {
	return w.cluster.Shoot.Status.TechnicalID
}
