package diff_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	testutil "github.com/argoproj/argo-cd/v3/test"
	argo "github.com/argoproj/argo-cd/v3/util/argo/diff"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
	"github.com/argoproj/argo-cd/v3/util/argo/testdata"
	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v3/util/cache/appstate"
)

func TestStateDiff(t *testing.T) {
	type diffConfigParams struct {
		ignores        []v1alpha1.ResourceIgnoreDifferences
		overrides      map[string]v1alpha1.ResourceOverride
		label          string
		trackingMethod string
		ignoreRoles    bool
	}
	defaultDiffConfigParams := func() *diffConfigParams {
		return &diffConfigParams{
			ignores:        []v1alpha1.ResourceIgnoreDifferences{},
			overrides:      map[string]v1alpha1.ResourceOverride{},
			label:          "",
			trackingMethod: "",
			ignoreRoles:    true,
		}
	}
	diffConfig := func(t *testing.T, params *diffConfigParams) argo.DiffConfig {
		t.Helper()
		diffConfig, err := argo.NewDiffConfigBuilder().
			WithDiffSettings(params.ignores, params.overrides, params.ignoreRoles, normalizers.IgnoreNormalizerOpts{}).
			WithTracking(params.label, params.trackingMethod).
			WithNoCache().
			Build()
		require.NoError(t, err)
		return diffConfig
	}
	type testcase struct {
		name                       string
		params                     func() *diffConfigParams
		desiredState               *unstructured.Unstructured
		liveState                  *unstructured.Unstructured
		expectedNormalizedReplicas int
		expectedPredictedReplicas  int
	}
	testcases := []*testcase{
		{
			name: "will normalize replica field if owned by trusted manager",
			params: func() *diffConfigParams {
				params := defaultDiffConfigParams()
				params.ignores = []v1alpha1.ResourceIgnoreDifferences{
					{
						Group:                 "*",
						Kind:                  "*",
						ManagedFieldsManagers: []string{"kube-controller-manager"},
					},
				}
				return params
			},
			desiredState:               testutil.YamlToUnstructured(testdata.DesiredDeploymentYaml),
			liveState:                  testutil.YamlToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml),
			expectedNormalizedReplicas: 1,
			expectedPredictedReplicas:  1,
		},
		{
			name: "will keep replica field not owned by trusted manager",
			params: func() *diffConfigParams {
				params := defaultDiffConfigParams()
				params.ignores = []v1alpha1.ResourceIgnoreDifferences{
					{
						Group:                 "*",
						Kind:                  "*",
						ManagedFieldsManagers: []string{"some-other-manager"},
					},
				}
				return params
			},
			desiredState:               testutil.YamlToUnstructured(testdata.DesiredDeploymentYaml),
			liveState:                  testutil.YamlToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml),
			expectedNormalizedReplicas: 2,
			expectedPredictedReplicas:  3,
		},
		{
			name: "will normalize replica field if configured with json pointers",
			params: func() *diffConfigParams {
				params := defaultDiffConfigParams()
				params.ignores = []v1alpha1.ResourceIgnoreDifferences{
					{
						Group:        "*",
						Kind:         "*",
						JSONPointers: []string{"/spec/replicas"},
					},
				}
				return params
			},
			desiredState:               testutil.YamlToUnstructured(testdata.DesiredDeploymentYaml),
			liveState:                  testutil.YamlToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml),
			expectedNormalizedReplicas: 1,
			expectedPredictedReplicas:  1,
		},
		{
			name: "will normalize replica field if configured with jq expression",
			params: func() *diffConfigParams {
				params := defaultDiffConfigParams()
				params.ignores = []v1alpha1.ResourceIgnoreDifferences{
					{
						Group:             "*",
						Kind:              "*",
						JQPathExpressions: []string{".spec.replicas"},
					},
				}
				return params
			},
			desiredState:               testutil.YamlToUnstructured(testdata.DesiredDeploymentYaml),
			liveState:                  testutil.YamlToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml),
			expectedNormalizedReplicas: 1,
			expectedPredictedReplicas:  1,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			dc := diffConfig(t, tc.params())

			// when
			result, err := argo.StateDiff(tc.liveState, tc.desiredState, dc)

			// then
			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.True(t, result.Modified)
			normalized := testutil.YamlToUnstructured(string(result.NormalizedLive))
			replicas, found, err := unstructured.NestedFloat64(normalized.Object, "spec", "replicas")
			require.NoError(t, err)
			assert.True(t, found)
			assert.InEpsilon(t, float64(tc.expectedNormalizedReplicas), replicas, 0.0001)
			predicted := testutil.YamlToUnstructured(string(result.PredictedLive))
			predictedReplicas, found, err := unstructured.NestedFloat64(predicted.Object, "spec", "replicas")
			require.NoError(t, err)
			assert.True(t, found)
			assert.InEpsilon(t, float64(tc.expectedPredictedReplicas), predictedReplicas, 0.0001)
		})
	}
}

func TestDiffConfigBuilder(t *testing.T) {
	type fixture struct {
		ignores        []v1alpha1.ResourceIgnoreDifferences
		overrides      map[string]v1alpha1.ResourceOverride
		label          string
		trackingMethod string
		noCache        bool
		ignoreRoles    bool
		appName        string
	}
	setup := func() *fixture {
		return &fixture{
			ignores:        []v1alpha1.ResourceIgnoreDifferences{},
			overrides:      make(map[string]v1alpha1.ResourceOverride),
			label:          "some-label",
			trackingMethod: "tracking-method",
			noCache:        true,
			ignoreRoles:    false,
			appName:        "application-name",
		}
	}
	t.Run("will build diff config successfully", func(t *testing.T) {
		// given
		f := setup()

		// when
		diffConfig, err := argo.NewDiffConfigBuilder().
			WithDiffSettings(f.ignores, f.overrides, f.ignoreRoles, normalizers.IgnoreNormalizerOpts{}).
			WithTracking(f.label, f.trackingMethod).
			WithNoCache().
			Build()

		// then
		require.NoError(t, err)
		require.NotNil(t, diffConfig)
		assert.Empty(t, diffConfig.Ignores())
		assert.Empty(t, diffConfig.Overrides())
		assert.Equal(t, f.label, diffConfig.AppLabelKey())
		assert.Equal(t, f.overrides, diffConfig.Overrides())
		assert.Equal(t, f.trackingMethod, diffConfig.TrackingMethod())
		assert.Equal(t, f.noCache, diffConfig.NoCache())
		assert.Equal(t, f.ignoreRoles, diffConfig.IgnoreAggregatedRoles())
		assert.Empty(t, diffConfig.AppName())
		assert.Nil(t, diffConfig.StateCache())
	})
	t.Run("will initialize ignore differences if nil is passed", func(t *testing.T) {
		// given
		f := setup()

		// when
		diffConfig, err := argo.NewDiffConfigBuilder().
			WithDiffSettings(nil, nil, f.ignoreRoles, normalizers.IgnoreNormalizerOpts{}).
			WithTracking(f.label, f.trackingMethod).
			WithNoCache().
			Build()

		// then
		require.NoError(t, err)
		require.NotNil(t, diffConfig)
		assert.Empty(t, diffConfig.Ignores())
		assert.Empty(t, diffConfig.Overrides())
		assert.Equal(t, f.label, diffConfig.AppLabelKey())
		assert.Equal(t, f.overrides, diffConfig.Overrides())
		assert.Equal(t, f.trackingMethod, diffConfig.TrackingMethod())
		assert.Equal(t, f.noCache, diffConfig.NoCache())
		assert.Equal(t, f.ignoreRoles, diffConfig.IgnoreAggregatedRoles())
	})
	t.Run("will return error if retrieving diff from cache an no appName configured", func(t *testing.T) {
		// given
		f := setup()

		// when
		diffConfig, err := argo.NewDiffConfigBuilder().
			WithDiffSettings(f.ignores, f.overrides, f.ignoreRoles, normalizers.IgnoreNormalizerOpts{}).
			WithTracking(f.label, f.trackingMethod).
			WithCache(&appstatecache.Cache{}, "").
			Build()

		// then
		require.Error(t, err)
		require.Nil(t, diffConfig)
	})
	t.Run("will return error if retrieving diff from cache and no stateCache configured", func(t *testing.T) {
		// given
		f := setup()

		// when
		diffConfig, err := argo.NewDiffConfigBuilder().
			WithDiffSettings(f.ignores, f.overrides, f.ignoreRoles, normalizers.IgnoreNormalizerOpts{}).
			WithTracking(f.label, f.trackingMethod).
			WithCache(nil, f.appName).
			Build()

		// then
		require.Error(t, err)
		require.Nil(t, diffConfig)
	})
}

func TestStateDiffWithTrackDifferences(t *testing.T) {
	type diffConfigParams struct {
		ignores        []v1alpha1.ResourceIgnoreDifferences
		overrides      map[string]v1alpha1.ResourceOverride
		label          string
		trackingMethod string
		ignoreRoles    bool
	}
	makeDiffConfig := func(t *testing.T, params *diffConfigParams) argo.DiffConfig {
		t.Helper()
		dc, err := argo.NewDiffConfigBuilder().
			WithDiffSettings(params.ignores, params.overrides, params.ignoreRoles, normalizers.IgnoreNormalizerOpts{}).
			WithTracking(params.label, params.trackingMethod).
			WithNoCache().
			Build()
		require.NoError(t, err)
		return dc
	}

	t.Run("will show diff for label added by tracked manager", func(t *testing.T) {
		// given: a live deployment with a manually-added label by kubectl-edit
		desiredState := testutil.YamlToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := testutil.YamlToUnstructured(testdata.LiveDeploymentWithTrackedLabelYaml)

		params := &diffConfigParams{
			ignores: []v1alpha1.ResourceIgnoreDifferences{},
			overrides: map[string]v1alpha1.ResourceOverride{
				"apps/Deployment": {
					TrackDifferences: v1alpha1.OverrideTrackDiff{
						ManagedFieldsManagers: []string{"kubectl-edit"},
					},
				},
			},
			ignoreRoles: true,
		}
		dc := makeDiffConfig(t, params)

		// when
		result, err := argo.StateDiff(liveState, desiredState, dc)

		// then
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Modified, "expected diff to show as modified when tracked manager added a label")

		// Verify the predicted live does NOT contain the manually-added label
		predicted := testutil.YamlToUnstructured(string(result.PredictedLive))
		labels, _, _ := unstructured.NestedStringMap(predicted.Object, "metadata", "labels")
		_, hasManualLabel := labels["manually-added-label"]
		assert.False(t, hasManualLabel, "predicted live should not contain the manually-added label")
	})

	t.Run("will not show diff when no tracked managers configured", func(t *testing.T) {
		// given: same live deployment with manually-added label, but no tracking configured
		desiredState := testutil.YamlToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := testutil.YamlToUnstructured(testdata.LiveDeploymentWithTrackedLabelYaml)

		params := &diffConfigParams{
			ignores:   []v1alpha1.ResourceIgnoreDifferences{},
			overrides: map[string]v1alpha1.ResourceOverride{},
		}
		dc := makeDiffConfig(t, params)

		// when
		result, err := argo.StateDiff(liveState, desiredState, dc)

		// then
		require.NoError(t, err)
		assert.NotNil(t, result)
		// Without trackDifferences, the manually added label is invisible
		// The diff may still be modified due to other fields (e.g. replicas=1 vs 3)
		// but the manually-added label should still be in predictedLive
		predicted := testutil.YamlToUnstructured(string(result.PredictedLive))
		labels, _, _ := unstructured.NestedStringMap(predicted.Object, "metadata", "labels")
		_, hasManualLabel := labels["manually-added-label"]
		assert.True(t, hasManualLabel, "without track differences, manually-added label should remain in predicted live")
	})

	t.Run("will show diff with wildcard track differences", func(t *testing.T) {
		// given: wildcard track differences
		desiredState := testutil.YamlToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := testutil.YamlToUnstructured(testdata.LiveDeploymentWithTrackedLabelYaml)

		params := &diffConfigParams{
			ignores: []v1alpha1.ResourceIgnoreDifferences{},
			overrides: map[string]v1alpha1.ResourceOverride{
				"*/*": {
					TrackDifferences: v1alpha1.OverrideTrackDiff{
						ManagedFieldsManagers: []string{"kubectl-edit"},
					},
				},
			},
		}
		dc := makeDiffConfig(t, params)

		// when
		result, err := argo.StateDiff(liveState, desiredState, dc)

		// then
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Modified, "expected diff to show as modified with wildcard track differences")

		// Verify the predicted live does NOT contain the manually-added label
		predicted := testutil.YamlToUnstructured(string(result.PredictedLive))
		labels, _, _ := unstructured.NestedStringMap(predicted.Object, "metadata", "labels")
		_, hasManualLabel := labels["manually-added-label"]
		assert.False(t, hasManualLabel, "predicted live should not contain the manually-added label")
	})

	t.Run("will not show diff for label added by non-tracked manager", func(t *testing.T) {
		// given: tracking a different manager than the one that added the label
		desiredState := testutil.YamlToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := testutil.YamlToUnstructured(testdata.LiveDeploymentWithTrackedLabelYaml)

		params := &diffConfigParams{
			ignores: []v1alpha1.ResourceIgnoreDifferences{},
			overrides: map[string]v1alpha1.ResourceOverride{
				"apps/Deployment": {
					TrackDifferences: v1alpha1.OverrideTrackDiff{
						ManagedFieldsManagers: []string{"some-other-manager"},
					},
				},
			},
		}
		dc := makeDiffConfig(t, params)

		// when
		result, err := argo.StateDiff(liveState, desiredState, dc)

		// then
		require.NoError(t, err)
		assert.NotNil(t, result)
		// The manually-added label should still be in predictedLive since we're tracking a different manager
		predicted := testutil.YamlToUnstructured(string(result.PredictedLive))
		labels, _, _ := unstructured.NestedStringMap(predicted.Object, "metadata", "labels")
		_, hasManualLabel := labels["manually-added-label"]
		assert.True(t, hasManualLabel, "manually-added label should remain since we're not tracking kubectl-edit")
	})

	t.Run("ignoreDifferences takes precedence over trackDifferences", func(t *testing.T) {
		// given: both ignoreDifferences and trackDifferences configured.
		// ignoreDifferences has a managedFieldsManager for "argocd" (which normalizes away
		// conflicting fields), while trackDifferences tracks "kubectl-edit".
		// The kubectl-edit label should still be detected even when ignore is active.
		desiredState := testutil.YamlToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := testutil.YamlToUnstructured(testdata.LiveDeploymentWithTrackedLabelYaml)

		params := &diffConfigParams{
			ignores: []v1alpha1.ResourceIgnoreDifferences{
				{
					Group:                 "*",
					Kind:                  "*",
					ManagedFieldsManagers: []string{"kube-controller-manager"},
				},
			},
			overrides: map[string]v1alpha1.ResourceOverride{
				"apps/Deployment": {
					TrackDifferences: v1alpha1.OverrideTrackDiff{
						ManagedFieldsManagers: []string{"kubectl-edit"},
					},
				},
			},
			ignoreRoles: true,
		}
		dc := makeDiffConfig(t, params)

		// when
		result, err := argo.StateDiff(liveState, desiredState, dc)

		// then
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Modified, "expected diff to show as modified with both ignore and track configured")

		// Verify the predicted live does NOT contain the manually-added label
		predicted := testutil.YamlToUnstructured(string(result.PredictedLive))
		labels, _, _ := unstructured.NestedStringMap(predicted.Object, "metadata", "labels")
		_, hasManualLabel := labels["manually-added-label"]
		assert.False(t, hasManualLabel, "predicted live should not contain the manually-added label even with ignoreDifferences active")
	})

	t.Run("multi-manager end-to-end: two managers each with distinct extra fields", func(t *testing.T) {
		// given: a live deployment with two tracked managers:
		//   kubectl-edit owns f:metadata.f:labels.f:manually-added-label
		//   kubectl-patch owns f:metadata.f:annotations.f:manual-annotation
		desiredState := testutil.YamlToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := testutil.YamlToUnstructured(testdata.LiveDeploymentWithMultiTrackedLabelsYaml)

		params := &diffConfigParams{
			ignores: []v1alpha1.ResourceIgnoreDifferences{},
			overrides: map[string]v1alpha1.ResourceOverride{
				"apps/Deployment": {
					TrackDifferences: v1alpha1.OverrideTrackDiff{
						ManagedFieldsManagers: []string{"kubectl-edit", "kubectl-patch"},
					},
				},
			},
			ignoreRoles: true,
		}
		dc := makeDiffConfig(t, params)

		// when
		result, err := argo.StateDiff(liveState, desiredState, dc)

		// then
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Modified, "expected diff to show as modified when two tracked managers added fields")

		// Verify neither the manually-added label nor the manual annotation appear in predicted live
		predicted := testutil.YamlToUnstructured(string(result.PredictedLive))
		labels, _, _ := unstructured.NestedStringMap(predicted.Object, "metadata", "labels")
		_, hasManualLabel := labels["manually-added-label"]
		assert.False(t, hasManualLabel, "predicted live should not contain the manually-added label from kubectl-edit")

		annotations, _, _ := unstructured.NestedStringMap(predicted.Object, "metadata", "annotations")
		_, hasManualAnnotation := annotations["manual-annotation"]
		assert.False(t, hasManualAnnotation, "predicted live should not contain the manual-annotation from kubectl-patch")
	})

	t.Run("cached-path: track diff still strips label from PredictedLive", func(t *testing.T) {
		// given: a pre-populated cache with a ResourceDiff that matches the live resource's ResourceVersion
		desiredState := testutil.YamlToUnstructured(testdata.DesiredDeploymentYaml)
		liveState := testutil.YamlToUnstructured(testdata.LiveDeploymentWithTrackedLabelYaml)

		// Build a cached diff that mirrors the initial (no-track) diff result.
		// The cache stores NormalizedLiveState and PredictedLiveState as JSON strings.
		// We compute a baseline diff first (without track), then cache it.
		baseParams := &diffConfigParams{
			ignores:   []v1alpha1.ResourceIgnoreDifferences{},
			overrides: map[string]v1alpha1.ResourceOverride{},
		}
		baseDC := makeDiffConfig(t, baseParams)
		baseResult, err := argo.StateDiff(liveState, desiredState, baseDC)
		require.NoError(t, err)

		// Create an in-memory cache and populate it
		stateCache := appstatecache.NewCache(
			cacheutil.NewCache(cacheutil.NewInMemoryCache(1*time.Hour)),
			1*time.Minute,
		)
		appName := "test-app"
		cachedDiffs := []*v1alpha1.ResourceDiff{
			{
				Group:               "apps",
				Kind:                "Deployment",
				Namespace:           "default",
				Name:                "kustomize-guestbook-ui",
				NormalizedLiveState: string(baseResult.NormalizedLive),
				PredictedLiveState:  string(baseResult.PredictedLive),
				ResourceVersion:     liveState.GetResourceVersion(), // must match for cache hit
				Modified:            baseResult.Modified,
			},
		}
		err = stateCache.SetAppManagedResources(appName, cachedDiffs)
		require.NoError(t, err)

		// Build DiffConfig with cache (NOT WithNoCache) and trackDifferences
		dc, err := argo.NewDiffConfigBuilder().
			WithDiffSettings(
				[]v1alpha1.ResourceIgnoreDifferences{},
				map[string]v1alpha1.ResourceOverride{
					"apps/Deployment": {
						TrackDifferences: v1alpha1.OverrideTrackDiff{
							ManagedFieldsManagers: []string{"kubectl-edit"},
						},
					},
				},
				true,
				normalizers.IgnoreNormalizerOpts{},
			).
			WithTracking("", "").
			WithCache(stateCache, appName).
			Build()
		require.NoError(t, err)

		// when
		result, err := argo.StateDiff(liveState, desiredState, dc)

		// then
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Modified, "expected diff to show as modified when tracked manager added a label (via cache)")

		// Verify the predicted live does NOT contain the manually-added label
		predicted := testutil.YamlToUnstructured(string(result.PredictedLive))
		labels, _, _ := unstructured.NestedStringMap(predicted.Object, "metadata", "labels")
		_, hasManualLabel := labels["manually-added-label"]
		assert.False(t, hasManualLabel, "predicted live should not contain the manually-added label (cached path)")
	})
}
