// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/terraformer"
)

const (
	// ModeManaged is mode value for managed resources.
	ModeManaged = "managed"
	// AttributeKeyId is the key for the id attribute
	AttributeKeyId = "id"
	// AttributeKeyName is the key for the name attribute
	AttributeKeyName = "name"
)

// TerraformState holds the unmarshalled terraformer state.
type TerraformState struct {
	Version          int                 `json:"version"`
	TerraformVersion string              `json:"terraform_version"`
	Serial           int                 `json:"serial"`
	Lineage          string              `json:"lineage"`
	Outputs          map[string]TFOutput `json:"outputs,omitempty"`
	Resources        []TFResource        `json:"resources,omitempty"`
}

// TFOutput holds the value and type for a terraformer state output variable.
type TFOutput struct {
	Value string `json:"value"`
	Type  string `json:"type"`
}

// TFResource holds the attributes of a terraformer state resource.
type TFResource struct {
	Mode      string `json:"mode"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	Instances []TFInstance
}

// TFInstance holds the attributes of a terraformer state resource instance.
type TFInstance struct {
	SchemaVersion       int                    `json:"schema_version"`
	Attributes          map[string]interface{} `json:"attributes,omitempty"`
	SensitiveAttributes []string               `json:"sensitive_attributes,omitempty"`
	Private             string                 `json:"private,omitempty"`
	Dependencies        []string               `json:"dependencies"`
}

// LoadTerraformStateFromConfigMapData loads and unmarshalls the state from a config map data.
func LoadTerraformStateFromConfigMapData(data map[string]string) (*TerraformState, error) {
	content := data["terraform.tfstate"]
	if content == "" {
		return nil, fmt.Errorf("key 'terraform.tfstate' not found")
	}

	return UnmarshalTerraformState([]byte(content))
}

// UnmarshalTerraformStateFromTerraformer unmarshalls the Terraformer state from the raw state.
func UnmarshalTerraformStateFromTerraformer(state *terraformer.RawState) (*TerraformState, error) {
	var (
		tfState *TerraformState
		err     error
		data    []byte
	)

	switch state.Encoding {
	case "base64":
		data, err = base64.StdEncoding.DecodeString(state.Data)
		if err != nil {
			return nil, fmt.Errorf("could not decode terraform raw state data: %w", err)
		}
	case "none":
		data = []byte(state.Data)
	default:
		return nil, fmt.Errorf("unknown encoding of Terraformer raw state: %s", state.Encoding)
	}
	if tfState, err = UnmarshalTerraformState(data); err != nil {
		return nil, fmt.Errorf("could not decode terraform state: %w", err)
	}
	return tfState, nil
}

// UnmarshalTerraformState unmarshalls the terraformer state from a byte array.
func UnmarshalTerraformState(data []byte) (*TerraformState, error) {
	state := &TerraformState{}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, err
	}
	return state, nil
}

// FindManagedResourceInstances finds all instances for a resource identified by type and name.
func (ts *TerraformState) FindManagedResourceInstances(tfType, resourceName string) []TFInstance {
	for i := range ts.Resources {
		resource := &ts.Resources[i]
		if resource.Mode == ModeManaged && resource.Type == tfType && resource.Name == resourceName {
			return resource.Instances
		}
	}
	return nil
}

// FindManagedResourcesByType finds all instances for all resources of the given type.
func (ts *TerraformState) FindManagedResourcesByType(tfType string) []*TFResource {
	var result []*TFResource
	for i := range ts.Resources {
		resource := &ts.Resources[i]
		if resource.Mode == ModeManaged && resource.Type == tfType {
			result = append(result, resource)
		}
	}
	return result
}

// GetManagedResourceInstanceID returns the value of the id attribute of the only instance of a resource identified by type and name.
// It returns nil if either the resource is not found or the resource has not exactly one instance.
func (ts *TerraformState) GetManagedResourceInstanceID(tfType, resourceName string) *string {
	return ts.GetManagedResourceInstanceAttribute(tfType, resourceName, AttributeKeyId)
}

// GetManagedResourceInstanceName returns the value of the name attribute of the only instance of a resource identified by type and name.
// It returns nil if either the resource is not found or the resource has not exactly one instance.
func (ts *TerraformState) GetManagedResourceInstanceName(tfType, resourceName string) *string {
	return ts.GetManagedResourceInstanceAttribute(tfType, resourceName, AttributeKeyName)
}

// GetManagedResourceInstanceAttribute returns the value of the given attribute keys of the only instance of a resource identified by type and name.
// It returns nil if either the resource is not found or the resource has not exactly one instance or the attribute key is not existing.
func (ts *TerraformState) GetManagedResourceInstanceAttribute(tfType, resourceName, attributeKey string) *string {
	instances := ts.FindManagedResourceInstances(tfType, resourceName)
	if len(instances) == 1 {
		if value, ok := AttributeAsString(instances[0].Attributes, attributeKey); ok {
			return &value
		}
	}
	return nil
}

// GetManagedResourceInstances returns a map resource name to instance id for all resources of the given type.
// Only resources are included which have exactly one instance.
func (ts *TerraformState) GetManagedResourceInstances(tfType string) map[string]string {
	result := map[string]string{}
	for _, item := range ts.FindManagedResourcesByType(tfType) {
		if len(item.Instances) == 1 {
			if value, ok := AttributeAsString(item.Instances[0].Attributes, AttributeKeyId); ok {
				result[item.Name] = value
			}
		}
	}
	return result
}

// AttributeAsString returns the string value for the given key. `found` is only true if the map contains the key and the value is a string.
func AttributeAsString(attributes map[string]interface{}, key string) (svalue string, found bool) {
	if attributes == nil {
		return
	}
	value, ok := attributes[key]
	if !ok {
		return
	}
	if s, ok := value.(string); ok {
		svalue = s
		found = true
	}
	return
}
