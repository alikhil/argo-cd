package diff

import (
	"slices"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/glob"
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
// Supports glob patterns in the override keys (e.g. "apps/*", "*/Deployment").
func (t *TrackDiffConfig) HasTrackDifference(group, kind string) (bool, *TrackDifference) {
	result := &TrackDifference{}
	found := false

	for key, ro := range t.overrides {
		if len(ro.TrackDifferences.ManagedFieldsManagers) == 0 {
			continue
		}
		overrideGroup, overrideKind := splitGroupKind(key)
		if glob.Match(overrideGroup, group) && glob.Match(overrideKind, kind) {
			mergeTrackDifferences(overrideToTrackDifference(ro), result)
			found = true
		}
	}

	if !found {
		return false, nil
	}
	return true, result
}

// splitGroupKind splits a "group/kind" key into its group and kind parts.
// If no "/" is present, returns ("", key).
func splitGroupKind(gk string) (string, string) {
	for i := range gk {
		if gk[i] == '/' {
			return gk[:i], gk[i+1:]
		}
	}
	return "", gk
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
