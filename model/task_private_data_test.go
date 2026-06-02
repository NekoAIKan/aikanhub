package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTaskPrivateDataRequestSnapshotRoundTrip(t *testing.T) {
	privateData := TaskPrivateData{
		RequestSnapshot: &TaskRequestSnapshot{
			Prompt:            "debug marker: orange robot walks left",
			Model:             "doubao-seedance-2-0-fast-260128",
			OriginModelName:   "doubao-seedance-2-0-fast-260128",
			UpstreamModelName: "doubao-seedance-2-0-fast-260128",
			Action:            "textGenerate",
			Platform:          "54",
			ChannelID:         17,
			ChannelType:       54,
			NormalizedRequest: json.RawMessage(`{"prompt":"debug marker: orange robot walks left"}`),
			UpstreamRequest:   json.RawMessage(`{"content":[{"type":"text","text":"debug marker: orange robot walks left"}]}`),
		},
	}

	value, err := privateData.Value()
	require.NoError(t, err)

	var restored TaskPrivateData
	require.NoError(t, restored.Scan(value))
	require.NotNil(t, restored.RequestSnapshot)
	require.Equal(t, privateData.RequestSnapshot.Prompt, restored.RequestSnapshot.Prompt)
	require.Equal(t, privateData.RequestSnapshot.Model, restored.RequestSnapshot.Model)
	require.JSONEq(t, string(privateData.RequestSnapshot.NormalizedRequest), string(restored.RequestSnapshot.NormalizedRequest))
	require.JSONEq(t, string(privateData.RequestSnapshot.UpstreamRequest), string(restored.RequestSnapshot.UpstreamRequest))
}
