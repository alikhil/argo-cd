package managedfields_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	arv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube/scheme"

	"github.com/argoproj/argo-cd/v3/util/argo/managedfields"
	"github.com/argoproj/argo-cd/v3/util/argo/testdata"
)

func TestNormalize(t *testing.T) {
	parser := scheme.StaticParser()
	t.Run("will remove conflicting fields if managed by trusted managers", func(t *testing.T) {
		// given
		desiredState := StrToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := StrToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml)
		trustedManagers := []string{"kube-controller-manager", "revision-history-manager"}
		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		// when
		liveResult, desiredResult, err := managedfields.Normalize(liveState, desiredState, trustedManagers, &pt)

		// then
		require.NoError(t, err)
		require.NotNil(t, liveResult)
		require.NotNil(t, desiredResult)
		desiredReplicas, ok, err := unstructured.NestedFloat64(desiredResult.Object, "spec", "replicas")
		assert.False(t, ok)
		require.NoError(t, err)
		liveReplicas, ok, err := unstructured.NestedFloat64(liveResult.Object, "spec", "replicas")
		assert.False(t, ok)
		require.NoError(t, err)
		assert.Zero(t, desiredReplicas)
		assert.Zero(t, liveReplicas)
		liveRevisionHistory, ok, err := unstructured.NestedFloat64(liveResult.Object, "spec", "revisionHistoryLimit")
		assert.False(t, ok)
		require.NoError(t, err)
		desiredRevisionHistory, ok, err := unstructured.NestedFloat64(desiredResult.Object, "spec", "revisionHistoryLimit")
		assert.False(t, ok)
		require.NoError(t, err)
		assert.Zero(t, desiredRevisionHistory)
		assert.Zero(t, liveRevisionHistory)
	})
	t.Run("will keep conflicting fields if not from trusted manager", func(t *testing.T) {
		// given
		desiredState := StrToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := StrToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml)
		trustedManagers := []string{"another-manager"}
		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		// when
		liveResult, desiredResult, err := managedfields.Normalize(liveState, desiredState, trustedManagers, &pt)

		// then
		require.NoError(t, err)
		validateNestedFloat64(t, float64(3), desiredResult, "spec", "replicas")
		validateNestedFloat64(t, float64(1), desiredResult, "spec", "revisionHistoryLimit")
		validateNestedFloat64(t, float64(2), liveResult, "spec", "replicas")
		validateNestedFloat64(t, float64(3), liveResult, "spec", "revisionHistoryLimit")
	})
	t.Run("no-op if live state is nil", func(t *testing.T) {
		// given
		desiredState := StrToUnstructured(testdata.DesiredDeploymentYaml)
		trustedManagers := []string{"kube-controller-manager"}
		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		// when
		liveResult, desiredResult, err := managedfields.Normalize(nil, desiredState, trustedManagers, &pt)

		// then
		require.NoError(t, err)
		assert.Nil(t, liveResult)
		assert.Nil(t, desiredResult)
		validateNestedFloat64(t, float64(3), desiredState, "spec", "replicas")
		validateNestedFloat64(t, float64(1), desiredState, "spec", "revisionHistoryLimit")
	})
	t.Run("no-op if desired state is nil", func(t *testing.T) {
		// given
		liveState := StrToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml)
		trustedManagers := []string{"kube-controller-manager"}
		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		// when
		liveResult, desiredResult, err := managedfields.Normalize(liveState, nil, trustedManagers, &pt)

		// then
		require.NoError(t, err)
		assert.Nil(t, liveResult)
		assert.Nil(t, desiredResult)
		validateNestedFloat64(t, float64(2), liveState, "spec", "replicas")
		validateNestedFloat64(t, float64(3), liveState, "spec", "revisionHistoryLimit")
	})
	t.Run("no-op if trusted manager list is empty", func(t *testing.T) {
		// given
		desiredState := StrToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := StrToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml)
		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		// when
		liveResult, desiredResult, err := managedfields.Normalize(liveState, desiredState, []string{}, &pt)

		// then
		require.NoError(t, err)
		assert.Nil(t, liveResult)
		assert.Nil(t, desiredResult)
		validateNestedFloat64(t, float64(3), desiredState, "spec", "replicas")
		validateNestedFloat64(t, float64(1), desiredState, "spec", "revisionHistoryLimit")
		validateNestedFloat64(t, float64(2), liveState, "spec", "replicas")
		validateNestedFloat64(t, float64(3), liveState, "spec", "revisionHistoryLimit")
	})
	t.Run("will normalize successfully inside a list", func(t *testing.T) {
		// given
		desiredState := StrToUnstructured(testdata.DesiredValidatingWebhookYaml)
		liveState := StrToUnstructured(testdata.LiveValidatingWebhookYaml)
		trustedManagers := []string{"external-secrets"}
		pt := parser.Type("io.k8s.api.admissionregistration.v1.ValidatingWebhookConfiguration")

		// when
		liveResult, desiredResult, err := managedfields.Normalize(liveState, desiredState, trustedManagers, &pt)

		// then
		require.NoError(t, err)
		require.NotNil(t, liveResult)
		require.NotNil(t, desiredResult)

		var vwcLive arv1.ValidatingWebhookConfiguration
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(liveResult.Object, &vwcLive)
		require.NoError(t, err)
		assert.Len(t, vwcLive.Webhooks, 1)
		assert.Empty(t, string(vwcLive.Webhooks[0].ClientConfig.CABundle))

		var vwcConfig arv1.ValidatingWebhookConfiguration
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(desiredResult.Object, &vwcConfig)
		require.NoError(t, err)
		assert.Len(t, vwcConfig.Webhooks, 1)
		assert.Empty(t, string(vwcConfig.Webhooks[0].ClientConfig.CABundle))
	})
	t.Run("does not fail if object fails validation schema", func(t *testing.T) {
		desiredState := StrToUnstructured(testdata.DesiredDeploymentYaml)
		require.NoError(t, unstructured.SetNestedField(desiredState.Object, "spec", "hello", "world"))
		liveState := StrToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml)

		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		_, _, err := managedfields.Normalize(liveState, desiredState, []string{}, &pt)
		require.NoError(t, err)
	})
}

func TestFindTrackedExtraFields(t *testing.T) {
	parser := scheme.StaticParser()

	t.Run("will find extra fields from tracked manager", func(t *testing.T) {
		// given
		desiredState := StrToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := StrToUnstructured(testdata.LiveDeploymentWithTrackedLabelYaml)
		trackedManagers := []string{"kubectl-edit"}
		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		// when
		extraFields, err := managedfields.FindTrackedExtraFields(liveState, desiredState, trackedManagers, &pt)

		// then
		require.NoError(t, err)
		require.NotNil(t, extraFields)
		assert.False(t, extraFields.Empty(), "expected to find extra fields from tracked manager")
	})

	t.Run("will not find extra fields from non-tracked manager", func(t *testing.T) {
		// given
		desiredState := StrToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := StrToUnstructured(testdata.LiveDeploymentWithTrackedLabelYaml)
		trackedManagers := []string{"another-manager"}
		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		// when
		extraFields, err := managedfields.FindTrackedExtraFields(liveState, desiredState, trackedManagers, &pt)

		// then
		require.NoError(t, err)
		require.NotNil(t, extraFields)
		assert.True(t, extraFields.Empty(), "expected no extra fields from non-tracked manager")
	})

	t.Run("no-op if tracked manager list is empty", func(t *testing.T) {
		// given
		desiredState := StrToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := StrToUnstructured(testdata.LiveDeploymentWithTrackedLabelYaml)
		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		// when
		extraFields, err := managedfields.FindTrackedExtraFields(liveState, desiredState, []string{}, &pt)

		// then
		require.NoError(t, err)
		require.NotNil(t, extraFields)
		assert.True(t, extraFields.Empty())
	})

	t.Run("no-op if live state is nil", func(t *testing.T) {
		// given
		desiredState := StrToUnstructured(testdata.DesiredDeploymentYaml)
		trackedManagers := []string{"kubectl-edit"}
		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		// when
		extraFields, err := managedfields.FindTrackedExtraFields(nil, desiredState, trackedManagers, &pt)

		// then
		require.NoError(t, err)
		require.NotNil(t, extraFields)
		assert.True(t, extraFields.Empty())
	})

	t.Run("no-op if desired state is nil", func(t *testing.T) {
		// given
		liveState := StrToUnstructured(testdata.LiveDeploymentWithTrackedLabelYaml)
		trackedManagers := []string{"kubectl-edit"}
		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		// when
		extraFields, err := managedfields.FindTrackedExtraFields(liveState, nil, trackedManagers, &pt)

		// then
		require.NoError(t, err)
		require.NotNil(t, extraFields)
		assert.True(t, extraFields.Empty())
	})

	t.Run("no-op if parseable type is nil", func(t *testing.T) {
		// given
		desiredState := StrToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := StrToUnstructured(testdata.LiveDeploymentWithTrackedLabelYaml)
		trackedManagers := []string{"kubectl-edit"}

		// when
		extraFields, err := managedfields.FindTrackedExtraFields(liveState, desiredState, trackedManagers, nil)

		// then
		require.NoError(t, err)
		require.NotNil(t, extraFields)
		assert.True(t, extraFields.Empty())
	})

	t.Run("will not report fields that exist in both tracked manager and config", func(t *testing.T) {
		// The kubectl-edit manager owns only f:metadata.f:labels.f:manually-added-label
		// which is NOT in the desired state. When we also track argocd, the argocd manager
		// owns many fields that ARE in the desired state. The Difference operation
		// should exclude fields that exist in both the tracked manager set AND the
		// config field set.
		desiredState := StrToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := StrToUnstructured(testdata.LiveDeploymentWithTrackedLabelYaml)
		// Track only kubectl-edit - it owns only the manually-added label
		trackedManagers := []string{"kubectl-edit"}
		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		// when
		extraFields, err := managedfields.FindTrackedExtraFields(liveState, desiredState, trackedManagers, &pt)

		// then
		require.NoError(t, err)
		require.NotNil(t, extraFields)
		// kubectl-edit only owns f:metadata.f:labels.f:manually-added-label
		// which is NOT in the desired state, so extraFields should be non-empty
		assert.False(t, extraFields.Empty(), "kubectl-edit manager owns a field not in config")

		// Now verify that when the tracked manager field IS in config, it's excluded
		// Add the manually-added-label to the desired state
		desiredWithLabel := desiredState.DeepCopy()
		labels := desiredWithLabel.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels["manually-added-label"] = "manual-value"
		desiredWithLabel.SetLabels(labels)

		extraFieldsWithLabel, err := managedfields.FindTrackedExtraFields(liveState, desiredWithLabel, trackedManagers, &pt)
		require.NoError(t, err)
		require.NotNil(t, extraFieldsWithLabel)
		// When the label IS in the desired state, the kubectl-edit manager's field
		// should be excluded from the extra set
		assert.True(t, extraFieldsWithLabel.Empty(), "expected no extra fields when tracked manager field is also in config")
	})
}

func validateNestedFloat64(t *testing.T, expected float64, obj *unstructured.Unstructured, fields ...string) {
	t.Helper()
	current := getNestedFloat64(t, obj, fields...)
	assert.InEpsilon(t, expected, current, 0.0001)
}

func getNestedFloat64(t *testing.T, obj *unstructured.Unstructured, fields ...string) float64 {
	t.Helper()
	current, ok, err := unstructured.NestedFloat64(obj.Object, fields...)
	assert.True(t, ok, "nested field not found")
	require.NoError(t, err)
	return current
}

func StrToUnstructured(jsonStr string) *unstructured.Unstructured {
	obj := make(map[string]any)
	err := yaml.Unmarshal([]byte(jsonStr), &obj)
	if err != nil {
		panic(err)
	}
	return &unstructured.Unstructured{Object: obj}
}

func TestFindTrackedExtraFields_ErrorPaths(t *testing.T) {
	parser := scheme.StaticParser()

	t.Run("error when config does not match parseable type", func(t *testing.T) {
		// given: a config with a completely wrong structure for Deployment type
		// (e.g., passing a Service schema but using Deployment parseable type)
		liveState := StrToUnstructured(testdata.LiveDeploymentWithTrackedLabelYaml)
		// Create a config that has invalid fields for the Deployment schema
		badConfig := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]any{"name": "test", "namespace": "default"},
			"spec": map[string]any{
				// replicas should be an integer, not a map — this will cause
				// pt.FromUnstructured to fail schema validation
				"replicas": map[string]any{"invalid": true},
			},
		}}
		trackedManagers := []string{"kubectl-edit"}
		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		// when
		_, err := managedfields.FindTrackedExtraFields(liveState, badConfig, trackedManagers, &pt)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error creating typedConfig for track diff")
	})

	t.Run("error when managed fields JSON is malformed", func(t *testing.T) {
		// given: a live resource where the tracked manager's FieldsV1 contains
		// malformed data that cannot be parsed by fieldpath.Set.FromJSON.
		// We inject this via the raw object map, bypassing SetManagedFields validation.
		liveState := StrToUnstructured(testdata.LiveDeploymentWithTrackedLabelYaml)
		desiredState := StrToUnstructured(testdata.DesiredDeploymentYaml)

		// Directly set managedFields with invalid fieldsV1 data in the raw map.
		liveState.Object["metadata"].(map[string]any)["managedFields"] = []any{
			map[string]any{
				"manager":    "kubectl-edit",
				"operation":  "Update",
				"apiVersion": "apps/v1",
				"fieldsType": "FieldsV1",
				"fieldsV1":   "invalid",
			},
		}

		trackedManagers := []string{"kubectl-edit"}
		pt := parser.Type("io.k8s.api.apps.v1.Deployment")

		// when
		_, err := managedfields.FindTrackedExtraFields(liveState, desiredState, trackedManagers, &pt)

		// then: The malformed fieldsV1 triggers a FromJSON parse error.
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error parsing managed fields for manager kubectl-edit")
	})
}
