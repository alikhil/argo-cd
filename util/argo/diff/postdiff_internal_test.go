package diff

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"

	enginediff "github.com/argoproj/argo-cd/gitops-engine/pkg/diff"
)

func TestPostDiffTrackChanges_MismatchedSliceLengths(t *testing.T) {
	// Build a minimal DiffConfig that has trackDifferences configured.
	dc, err := NewDiffConfigBuilder().
		WithDiffSettings(
			[]v1alpha1.ResourceIgnoreDifferences{},
			map[string]v1alpha1.ResourceOverride{
				"apps/Deployment": {
					TrackDifferences: v1alpha1.OverrideTrackDiff{
						ManagedFieldsManagers: []string{"kubectl-edit"},
					},
				},
			},
			false,
			normalizers.IgnoreNormalizerOpts{},
		).
		WithTracking("", "").
		WithNoCache().
		Build()
	assert.NoError(t, err)

	makeObj := func() *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]any{"name": "test", "namespace": "default"},
		}}
	}

	liveJSON, _ := json.Marshal(makeObj().Object)

	// result has 2 diffs, but only 1 target — mismatched
	result := &enginediff.DiffResultList{
		Diffs: []enginediff.DiffResult{
			{NormalizedLive: liveJSON, PredictedLive: liveJSON},
			{NormalizedLive: liveJSON, PredictedLive: liveJSON},
		},
	}
	targets := []*unstructured.Unstructured{makeObj()}        // len=1
	normLives := []*unstructured.Unstructured{makeObj()}      // len=1
	originalLives := []*unstructured.Unstructured{makeObj()}  // len=1

	// Should return cleanly without panicking
	assert.NotPanics(t, func() {
		postDiffTrackChanges(result, originalLives, normLives, targets, dc)
	})
}

func TestPostDiffTrackChanges_ServerSideDiffSkipped(t *testing.T) {
	// Build a diffConfig directly (bypassing builder validation) with server-side diff enabled.
	dc := &diffConfig{
		overrides: map[string]v1alpha1.ResourceOverride{
			"apps/Deployment": {
				TrackDifferences: v1alpha1.OverrideTrackDiff{
					ManagedFieldsManagers: []string{"kubectl-edit"},
				},
			},
		},
		noCache:        true,
		serverSideDiff: true,
	}

	makeObj := func() *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]any{"name": "test", "namespace": "default"},
		}}
	}

	liveJSON, _ := json.Marshal(makeObj().Object)

	result := &enginediff.DiffResultList{
		Diffs: []enginediff.DiffResult{
			{NormalizedLive: liveJSON, PredictedLive: liveJSON, Modified: false},
		},
	}
	targets := []*unstructured.Unstructured{makeObj()}
	normLives := []*unstructured.Unstructured{makeObj()}
	originalLives := []*unstructured.Unstructured{makeObj()}

	// postDiffTrackChanges should be a no-op: Modified stays false.
	postDiffTrackChanges(result, originalLives, normLives, targets, dc)
	assert.False(t, result.Modified, "server-side diff should skip track diff processing")
	assert.False(t, result.Diffs[0].Modified, "individual diff should not be modified")
}
