package validation

import (
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"gopkg.in/inf.v0"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	openstackclient "github.com/gardener/gardener-extension-provider-openstack/pkg/openstack/client"
)

// ValidateWorkerConfig validates the providerConfig section of a Worker resource.
func ValidateWorkerConfig(worker *core.Worker, workerConfig *api.WorkerConfig, cloudProfileConfig *api.CloudProfileConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateServerGroup(worker, workerConfig.ServerGroup, cloudProfileConfig, fldPath.Child("serverGroup"))...)
	allErrs = append(allErrs, ValidateNodeTemplate(workerConfig.NodeTemplate, fldPath.Child("nodeTemplate"))...)
	allErrs = append(allErrs, ValidateMachineLabels(worker, workerConfig, fldPath.Child("machineLabels"))...)

	return allErrs
}

func ValidateServerGroup(worker *core.Worker, sg *api.ServerGroup, cloudProfileConfig *api.CloudProfileConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if sg == nil {
		return allErrs
	}

	if sg.Policy == "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("policy"), sg.Policy, "policy field cannot be empty"))
		return allErrs
	}

	isPolicyMatching := func() bool {
		if cloudProfileConfig == nil {
			return false
		}

		for _, policy := range cloudProfileConfig.ServerGroupPolicies {
			if policy == sg.Policy {
				return true
			}
		}
		return false
	}()

	if !isPolicyMatching {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("policy"), sg.Policy, "no matching server group policy found in cloudprofile"))
		return allErrs
	}

	if len(worker.Zones) > 1 && sg.Policy == openstackclient.ServerGroupPolicyAffinity {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("policy"), fmt.Sprintf("using %q policy with multiple availability zones is not allowed", openstackclient.ServerGroupPolicyAffinity)))
	}

	return allErrs
}

func ValidateNodeTemplate(nodeTemplate *extensionsv1alpha1.NodeTemplate, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if nodeTemplate == nil {
		return nil
	}
	for _, capacityAttribute := range []corev1.ResourceName{corev1.ResourceCPU, "gpu", corev1.ResourceMemory} {
		value, ok := nodeTemplate.Capacity[capacityAttribute]
		if !ok {
			// core resources such as "cpu", "gpu", "memory" need not all be explicitly specified in workerConfig.NodeTemplate.
			// Will fall back to the worker pool's node template if missing.
			continue
		}
		allErrs = append(allErrs, validateResourceQuantityValue(capacityAttribute, value, fldPath.Child("capacity").Child(string(capacityAttribute)))...)
	}

	for capacityAttribute, value := range nodeTemplate.VirtualCapacity {
		// extended resources are required to be whole numbers https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#consuming-extended-resources
		allErrs = append(allErrs, validateResourceQuantityWholeNumber(capacityAttribute, value, fldPath.Child("virtualCapacity").Child(string(capacityAttribute)))...)
	}

	return allErrs
}

func validateResourceQuantityWholeNumber(key corev1.ResourceName, value resource.Quantity, fldPath *field.Path) field.ErrorList {
	allErrs := validateResourceQuantityValue(key, value, fldPath)

	dec := value.AsDec()
	var roundedDec inf.Dec
	if roundedDec.Round(dec, 0, inf.RoundExact) == nil {
		allErrs = append(allErrs, field.Invalid(fldPath, value.String(), fmt.Sprintf("%s value must be a whole number", key)))
	}

	return allErrs
}

func validateResourceQuantityValue(key corev1.ResourceName, value resource.Quantity, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if value.Cmp(resource.Quantity{}) < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, value.String(), fmt.Sprintf("%s value must not be negative", key)))
	}

	return allErrs
}

func ValidateMachineLabels(worker *core.Worker, workerConfig *api.WorkerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	machineLabelNames := sets.New[string]()
	for i, ml := range workerConfig.MachineLabels {
		idxPath := fldPath.Index(i)
		allErrs = append(allErrs, validateResourceName(ml.Name, idxPath.Child("name"))...)
		allErrs = append(allErrs, validateResourceName(ml.Value, idxPath.Child("value"))...)
		if machineLabelNames.Has(ml.Name) {
			allErrs = append(allErrs, field.Duplicate(idxPath.Child("name"), ml.Name))
		} else if _, found := worker.Labels[ml.Name]; found {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("name"), ml.Name, "label name already defined as pool label"))
		} else {
			machineLabelNames.Insert(ml.Name)
		}
	}

	return allErrs
}
