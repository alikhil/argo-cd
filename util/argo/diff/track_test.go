package diff_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo/diff"
)

func TestTrackDiffConfig_HasTrackDifference(t *testing.T) {
	getOverride := func(gk string) map[string]v1alpha1.ResourceOverride {
		return map[string]v1alpha1.ResourceOverride{
			gk: {
				TrackDifferences: v1alpha1.OverrideTrackDiff{
					ManagedFieldsManagers: []string{"kubectl-edit", "kubectl-client-side-apply"},
				},
			},
		}
	}

	t.Run("will return track diffs from resource override", func(t *testing.T) {
		// given
		gk := "apps/Deployment"
		override := getOverride(gk)
		trackConfig := diff.NewTrackDiffConfig(override)

		// when
		ok, actual := trackConfig.HasTrackDifference("apps", "Deployment")

		// then
		assert.True(t, ok)
		require.NotNil(t, actual)
		assert.Equal(t, []string{"kubectl-edit", "kubectl-client-side-apply"}, actual.ManagedFieldsManagers)
	})

	t.Run("will return track diffs from wildcard override", func(t *testing.T) {
		// given
		gk := "*/*"
		override := getOverride(gk)
		trackConfig := diff.NewTrackDiffConfig(override)

		// when
		ok, actual := trackConfig.HasTrackDifference("apps", "Deployment")

		// then
		assert.True(t, ok)
		require.NotNil(t, actual)
		assert.Equal(t, []string{"kubectl-edit", "kubectl-client-side-apply"}, actual.ManagedFieldsManagers)
	})

	t.Run("will merge track diffs from specific and wildcard overrides", func(t *testing.T) {
		// given
		override := map[string]v1alpha1.ResourceOverride{
			"apps/Deployment": {
				TrackDifferences: v1alpha1.OverrideTrackDiff{
					ManagedFieldsManagers: []string{"kubectl-edit"},
				},
			},
			"*/*": {
				TrackDifferences: v1alpha1.OverrideTrackDiff{
					ManagedFieldsManagers: []string{"kubectl-client-side-apply"},
				},
			},
		}
		trackConfig := diff.NewTrackDiffConfig(override)

		// when
		ok, actual := trackConfig.HasTrackDifference("apps", "Deployment")

		// then
		assert.True(t, ok)
		require.NotNil(t, actual)
		assert.ElementsMatch(t, []string{"kubectl-edit", "kubectl-client-side-apply"}, actual.ManagedFieldsManagers)
	})

	t.Run("will deduplicate managers when merging", func(t *testing.T) {
		// given
		override := map[string]v1alpha1.ResourceOverride{
			"apps/Deployment": {
				TrackDifferences: v1alpha1.OverrideTrackDiff{
					ManagedFieldsManagers: []string{"kubectl-edit", "shared-manager"},
				},
			},
			"*/*": {
				TrackDifferences: v1alpha1.OverrideTrackDiff{
					ManagedFieldsManagers: []string{"shared-manager", "kubectl-client-side-apply"},
				},
			},
		}
		trackConfig := diff.NewTrackDiffConfig(override)

		// when
		ok, actual := trackConfig.HasTrackDifference("apps", "Deployment")

		// then
		assert.True(t, ok)
		require.NotNil(t, actual)
		assert.ElementsMatch(t, []string{"kubectl-edit", "shared-manager", "kubectl-client-side-apply"}, actual.ManagedFieldsManagers)
	})

	t.Run("no track diffs if resource group does not match", func(t *testing.T) {
		// given
		gk := "apps/Deployment"
		override := getOverride(gk)
		trackConfig := diff.NewTrackDiffConfig(override)

		// when
		ok, actual := trackConfig.HasTrackDifference("batch", "Job")

		// then
		assert.False(t, ok)
		assert.Nil(t, actual)
	})

	t.Run("no track diffs if no overrides configured", func(t *testing.T) {
		// given
		trackConfig := diff.NewTrackDiffConfig(map[string]v1alpha1.ResourceOverride{})

		// when
		ok, actual := trackConfig.HasTrackDifference("apps", "Deployment")

		// then
		assert.False(t, ok)
		assert.Nil(t, actual)
	})

	t.Run("no track diffs if override has no ManagedFieldsManagers", func(t *testing.T) {
		// given
		override := map[string]v1alpha1.ResourceOverride{
			"apps/Deployment": {
				TrackDifferences: v1alpha1.OverrideTrackDiff{
					ManagedFieldsManagers: []string{},
				},
			},
		}
		trackConfig := diff.NewTrackDiffConfig(override)

		// when
		ok, actual := trackConfig.HasTrackDifference("apps", "Deployment")

		// then
		assert.False(t, ok)
		assert.Nil(t, actual)
	})

	t.Run("no track diffs with nil overrides", func(t *testing.T) {
		// given
		trackConfig := diff.NewTrackDiffConfig(nil)

		// when
		ok, actual := trackConfig.HasTrackDifference("apps", "Deployment")

		// then
		assert.False(t, ok)
		assert.Nil(t, actual)
	})

	// Glob pattern tests
	t.Run("will match with group glob pattern apps/*", func(t *testing.T) {
		// given
		override := map[string]v1alpha1.ResourceOverride{
			"apps/*": {
				TrackDifferences: v1alpha1.OverrideTrackDiff{
					ManagedFieldsManagers: []string{"kubectl-edit"},
				},
			},
		}
		trackConfig := diff.NewTrackDiffConfig(override)

		// when
		ok, actual := trackConfig.HasTrackDifference("apps", "Deployment")

		// then
		assert.True(t, ok)
		require.NotNil(t, actual)
		assert.Equal(t, []string{"kubectl-edit"}, actual.ManagedFieldsManagers)

		// should also match StatefulSet in the same group
		ok, actual = trackConfig.HasTrackDifference("apps", "StatefulSet")
		assert.True(t, ok)
		require.NotNil(t, actual)

		// should not match a different group
		ok, _ = trackConfig.HasTrackDifference("batch", "Job")
		assert.False(t, ok)
	})

	t.Run("will match with kind glob pattern */Deployment", func(t *testing.T) {
		// given
		override := map[string]v1alpha1.ResourceOverride{
			"*/Deployment": {
				TrackDifferences: v1alpha1.OverrideTrackDiff{
					ManagedFieldsManagers: []string{"kubectl-edit"},
				},
			},
		}
		trackConfig := diff.NewTrackDiffConfig(override)

		// when
		ok, actual := trackConfig.HasTrackDifference("apps", "Deployment")

		// then
		assert.True(t, ok)
		require.NotNil(t, actual)
		assert.Equal(t, []string{"kubectl-edit"}, actual.ManagedFieldsManagers)

		// should match Deployment in any group
		ok, actual = trackConfig.HasTrackDifference("extensions", "Deployment")
		assert.True(t, ok)
		require.NotNil(t, actual)

		// should not match other kinds
		ok, _ = trackConfig.HasTrackDifference("apps", "StatefulSet")
		assert.False(t, ok)
	})

	t.Run("will merge glob and exact matches", func(t *testing.T) {
		// given
		override := map[string]v1alpha1.ResourceOverride{
			"apps/Deployment": {
				TrackDifferences: v1alpha1.OverrideTrackDiff{
					ManagedFieldsManagers: []string{"kubectl-edit"},
				},
			},
			"apps/*": {
				TrackDifferences: v1alpha1.OverrideTrackDiff{
					ManagedFieldsManagers: []string{"kubectl-client-side-apply"},
				},
			},
		}
		trackConfig := diff.NewTrackDiffConfig(override)

		// when
		ok, actual := trackConfig.HasTrackDifference("apps", "Deployment")

		// then - both should match and merge
		assert.True(t, ok)
		require.NotNil(t, actual)
		assert.ElementsMatch(t, []string{"kubectl-edit", "kubectl-client-side-apply"}, actual.ManagedFieldsManagers)
	})
}
