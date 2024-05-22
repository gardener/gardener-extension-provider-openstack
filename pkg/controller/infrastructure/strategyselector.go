package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/go-logr/logr"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/v1alpha1"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
)

// Reconciler is an interface for the infrastructure reconciliation.
type Reconciler interface {
	// Reconcile manages infrastructure resources according to desired spec.
	Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error
	// Delete removes any created infrastructure resource on the provider.
	Delete(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error
	// Restore restores the infrastructure after a control plane migration. Effectively it performs a recovery of data from the infrastructure.status.state and
	// proceeds to reconcile.
	Restore(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error
}

// ReconcilerFactory can construct the different infrastructure reconciler implementations.
type ReconcilerFactory interface {
	Build(useFlow bool) (Reconciler, error)
}

// ReconcilerFactoryImpl is an implementation of a ReconcilerFactory
type ReconcilerFactoryImpl struct {
	ctx   context.Context
	log   logr.Logger
	a     *actuator
	infra *extensionsv1alpha1.Infrastructure
}

// Build builds the Reconciler according to the arguments.
func (f ReconcilerFactoryImpl) Build(useFlow bool) (Reconciler, error) {
	if useFlow {
		reconciler, err := NewFlowReconciler(f.a.client, f.a.restConfig, f.log, f.a.disableProjectedTokenMount)
		if err != nil {
			return nil, fmt.Errorf("failed to init flow reconciler: %w", err)
		}
		return reconciler, nil
	}

	return NewTerraformReconciler(f.a.client, f.a.restConfig, f.log, f.a.disableProjectedTokenMount), nil
}

// SelectorFunc decides the reconciler used.
type SelectorFunc func(*extensionsv1alpha1.Infrastructure, *extensions.Cluster) (bool, error)

// OnReconcile returns true if the operation should use the Flow for the given cluster.
func OnReconcile(infra *extensionsv1alpha1.Infrastructure, _ *extensions.Cluster) (bool, error) {
	hasState, err := hasFlowState(infra.Status.State)
	if err != nil {
		return false, err
	}
	return hasState || GetFlowAnnotationValue(infra), nil
}

// OnDelete returns true if the operation should use the Flow deletion for the given cluster.
func OnDelete(infra *extensionsv1alpha1.Infrastructure, _ *extensions.Cluster) (bool, error) {
	return hasFlowState(infra.Status.State)
}

// OnRestore decides the reconciler used on migration.
var OnRestore = OnDelete

// GetFlowAnnotationValue returns the boolean value of the expected flow annotation. Returns false if the annotation was not found, if it couldn't be converted to bool,
// or had a "false" value.
func GetFlowAnnotationValue(o v1.Object) bool {
	if annotations := o.GetAnnotations(); annotations != nil {
		for _, k := range openstack.ValidFlowAnnotations {
			if str, ok := annotations[k]; ok {
				if v, err := strconv.ParseBool(str); err != nil {
					return false
				} else {
					return v
				}
			}
		}
	}
	return false
}

func hasFlowState(state *runtime.RawExtension) (bool, error) {
	if state == nil {
		return false, nil
	}

	flowState := runtime.TypeMeta{}
	stateJson, err := state.MarshalJSON()
	if err != nil {
		return false, err
	}

	if err := json.Unmarshal(stateJson, &flowState); err != nil {
		return false, err
	}

	if flowState.GroupVersionKind().GroupVersion() == v1alpha1.SchemeGroupVersion {
		return true, nil
	}

	return false, nil
}
