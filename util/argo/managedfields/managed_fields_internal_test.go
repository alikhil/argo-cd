package managedfields

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube/scheme"
	"github.com/argoproj/argo-cd/v3/util/argo/testdata"
	"sigs.k8s.io/yaml"
)

func strToUnstructured(yamlStr string) *unstructured.Unstructured {
	obj := make(map[string]any)
	err := yaml.Unmarshal([]byte(yamlStr), &obj)
	if err != nil {
		panic(err)
	}
	return &unstructured.Unstructured{Object: obj}
}

func TestFindTrackedExtraFields_MalformedFieldsV1(t *testing.T) {
	parser := scheme.StaticParser()

	t.Run("error when FieldsV1.Raw is malformed JSON", func(t *testing.T) {
		// given: a live resource where the tracked manager has malformed FieldsV1 data.
		// We inject the malformed data directly into the underlying Object map to bypass
		// SetManagedFields validation. This simulates corrupt data from the API server.
		liveState := strToUnstructured(testdata.LiveDeploymentWithTrackedLabelYaml)
		desiredState := strToUnstructured(testdata.DesiredDeploymentYaml)
		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		// Inject directly into the raw object map. GetManagedFields() will attempt to
		// convert this via runtime.DefaultUnstructuredConverter.FromUnstructured, and
		// the malformed fieldsV1 value will either:
		// a) cause a conversion error (returning nil from GetManagedFields), or
		// b) produce a ManagedFieldsEntry with nil/empty FieldsV1
		// In both cases, the function should not panic.
		metadata := liveState.Object["metadata"].(map[string]any)
		metadata["managedFields"] = []any{
			map[string]any{
				"manager":    "kubectl-edit",
				"operation":  "Update",
				"apiVersion": "apps/v1",
				"fieldsType": "FieldsV1",
				// Invalid: fieldsV1 should be a map, not a string
				"fieldsV1": "invalid",
			},
		}

		// when
		_, err := FindTrackedExtraFields(liveState, desiredState, []string{"kubectl-edit"}, &pt)

		// then: The malformed fieldsV1 triggers a FromJSON parse error.
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error parsing managed fields for manager kubectl-edit")
	})

	t.Run("FieldsV1 is nil for tracked manager is skipped", func(t *testing.T) {
		// given: a live resource where the tracked manager has nil FieldsV1
		liveState := strToUnstructured(testdata.LiveDeploymentWithTrackedLabelYaml)
		desiredState := strToUnstructured(testdata.DesiredDeploymentYaml)
		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		liveState.SetManagedFields([]metav1.ManagedFieldsEntry{
			{
				Manager:    "kubectl-edit",
				Operation:  metav1.ManagedFieldsOperationUpdate,
				APIVersion: "apps/v1",
				FieldsType: "FieldsV1",
				FieldsV1:   nil, // nil FieldsV1 should be skipped
			},
		})

		// when
		result, err := FindTrackedExtraFields(liveState, desiredState, []string{"kubectl-edit"}, &pt)

		// then
		require.NoError(t, err)
		assert.True(t, result.Empty(), "expected empty result when FieldsV1 is nil")
	})
}
