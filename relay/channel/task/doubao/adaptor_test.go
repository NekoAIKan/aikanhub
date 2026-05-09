package doubao

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestConvertToRequestPayloadUsesTopLevelDurationAndResolution(t *testing.T) {
	raw := []byte(`{
		"model": "doubao-seedance-2-0-260128",
		"prompt": "切换到面部特写，角色露出邪魅一笑。",
		"metadata": {
			"content": [
				{
					"type": "image_url",
					"image_url": {
						"url": "https://example.com/first-frame.png"
					},
					"role": "first_frame"
				}
			],
			"audit_image": true
		},
		"duration": 4,
		"resolution": "480p"
	}`)

	var req relaycommon.TaskSubmitReq
	require.NoError(t, common.Unmarshal(raw, &req))

	got, err := (&TaskAdaptor{}).convertToRequestPayload(&req)
	require.NoError(t, err)

	require.Equal(t, "doubao-seedance-2-0-260128", got.Model)
	require.Equal(t, "480p", got.Resolution)
	require.NotNil(t, got.Duration)
	require.Equal(t, 4, int(*got.Duration))

	require.Len(t, got.Content, 2)
	require.Equal(t, "image_url", got.Content[0].Type)
	require.Equal(t, "first_frame", got.Content[0].Role)
	require.NotNil(t, got.Content[0].ImageURL)
	require.Equal(t, "https://example.com/first-frame.png", got.Content[0].ImageURL.URL)
	require.Equal(t, "text", got.Content[1].Type)
	require.Equal(t, "切换到面部特写，角色露出邪魅一笑。", got.Content[1].Text)
}

func TestConvertToRequestPayloadDurationSources(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{
			name: "top level duration number",
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"test","duration":4}`,
			want: 4,
		},
		{
			name: "top level duration string",
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"test","duration":"6"}`,
			want: 6,
		},
		{
			name: "legacy seconds string",
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"test","seconds":"8"}`,
			want: 8,
		},
		{
			name: "duration wins over seconds",
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"test","duration":4,"seconds":"8"}`,
			want: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req relaycommon.TaskSubmitReq
			require.NoError(t, common.Unmarshal([]byte(tt.body), &req))

			got, err := (&TaskAdaptor{}).convertToRequestPayload(&req)
			require.NoError(t, err)
			require.NotNil(t, got.Duration)
			require.Equal(t, tt.want, int(*got.Duration))
		})
	}
}
