package diff

import (
	"fmt"
	"slices"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// TrackDiffConfig holds the track difference configurations defined in argocd-cm
// resource overrides.
type TrackDiffConfig struct {
	overrides map[string]v1alpha1.ResourceOverride
}

// TrackDifference holds the configurations to be used while tracking differences
// from live and desired states.
type TrackDifference struct {
	// ManagedFieldsManagers is a list of field managers whose changes should be tracked.
	ManagedFieldsManagers []string
}

// NewTrackDiffConfig creates a new TrackDiffConfig.
func NewTrackDiffConfig(overrides map[string]v1alpha1.ResourceOverride) *TrackDiffConfig {
	return &TrackDiffConfig{
		overrides: overrides,
	}
}

// HasTrackDifference will verify if the provided resource identifiers have any track
// difference configurations associated with them. It checks system-level track difference
// configurations for the current group/kind and wildcard overrides.
func (t *TrackDiffConfig) HasTrackDifference(group, kind, name, namespace string) (bool, *TrackDifference) {
	result := &TrackDifference{}
	found := false

	ro, ok := t.overrides[fmt.Sprintf("%s/%s", group, kind)]
	if ok && len(ro.TrackDifferences.ManagedFieldsManagers) > 0 {
		mergeTrackDifferences(overrideToTrackDifference(ro), result)
		found = true
	}
	wildOverride, ok := t.overrides["*/*"]
	if ok && len(wildOverride.TrackDifferences.ManagedFieldsManagers) > 0 {
		mergeTrackDifferences(overrideToTrackDifference(wildOverride), result)
		found = true
	}

	if !found {
		return false, nil
	}
	return true, result
}

func overrideToTrackDifference(override v1alpha1.ResourceOverride) *TrackDifference {
	return &TrackDifference{
		ManagedFieldsManagers: override.TrackDifferences.ManagedFieldsManagers,
	}
}

// mergeTrackDifferences will merge all track managers from 'from' into 'target',
// skipping duplicates.
func mergeTrackDifferences(from *TrackDifference, target *TrackDifference) {
	for _, manager := range from.ManagedFieldsManagers {
		if !slices.Contains(target.ManagedFieldsManagers, manager) {
			target.ManagedFieldsManagers = append(target.ManagedFieldsManagers, manager)
		}
	}
}
