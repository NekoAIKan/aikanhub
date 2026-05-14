package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestImagesFromMetadataContent locks the OpenAI-style metadata.content
// → req.Images hydration. Without this helper, downstream task adaptors
// (pixverse i2v, doubao referenceGenerate) read req.Images as empty
// when callers pass image references inside metadata.content and reject
// the request as missing image input.
func TestImagesFromMetadataContent(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		want     []string
	}{
		{name: "nil metadata", metadata: nil, want: nil},
		{
			name:     "no content key",
			metadata: map[string]any{"quality": "720p"},
			want:     nil,
		},
		{
			name: "single image_url with object form",
			metadata: map[string]any{
				"content": []any{
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://x/a.jpg"}},
				},
			},
			want: []string{"https://x/a.jpg"},
		},
		{
			name: "multiple images preserve order",
			metadata: map[string]any{
				"content": []any{
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://x/first.jpg"}},
					map[string]any{"type": "text", "text": "between"},
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://x/second.jpg"}},
				},
			},
			want: []string{"https://x/first.jpg", "https://x/second.jpg"},
		},
		{
			name: "image_url as bare string is accepted",
			metadata: map[string]any{
				"content": []any{
					map[string]any{"type": "image_url", "image_url": "https://x/legacy.jpg"},
				},
			},
			want: []string{"https://x/legacy.jpg"},
		},
		{
			name: "non-image entries skipped",
			metadata: map[string]any{
				"content": []any{
					map[string]any{"type": "text", "text": "skip me"},
					map[string]any{"type": "video_url", "video_url": map[string]any{"url": "https://x/v.mp4"}},
				},
			},
			want: nil,
		},
		{
			name: "image_url presence without explicit type still counts",
			metadata: map[string]any{
				"content": []any{
					map[string]any{"image_url": map[string]any{"url": "https://x/i.jpg"}},
				},
			},
			want: []string{"https://x/i.jpg"},
		},
		{
			name: "[]map content shape (post-marshal)",
			metadata: map[string]any{
				"content": []map[string]any{
					{"type": "image_url", "image_url": map[string]any{"url": "https://x/m.jpg"}},
				},
			},
			want: []string{"https://x/m.jpg"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, imagesFromMetadataContent(tc.metadata))
		})
	}
}

func TestKnownTaskFieldsIncludesAspectRatioAlias(t *testing.T) {
	require.True(t, isKnownTaskField("ratio"))
	require.True(t, isKnownTaskField("aspect_ratio"))
}
