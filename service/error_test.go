package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResetStatusCode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		statusCode       int
		statusCodeConfig string
		expectedCode     int
	}{
		{
			name:             "map string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"503"}`,
			expectedCode:     503,
		},
		{
			name:             "map int value",
			statusCode:       429,
			statusCodeConfig: `{"429":503}`,
			expectedCode:     503,
		},
		{
			name:             "skip invalid string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"bad-code"}`,
			expectedCode:     429,
		},
		{
			name:             "skip status code 200",
			statusCode:       200,
			statusCodeConfig: `{"200":503}`,
			expectedCode:     200,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			newAPIError := &types.NewAPIError{
				StatusCode: tc.statusCode,
			}
			ResetStatusCode(newAPIError, tc.statusCodeConfig)
			require.Equal(t, tc.expectedCode, newAPIError.StatusCode)
		})
	}
}

// TestClaudeErrorWrapper_PreservesUpstreamMessage guards CLAUDE.md Rule 31:
// when the wrapped error contains network keywords (post/dial/http) the
// wrapper must mask URL components but MUST NOT replace the message with a
// synthetic string. The legacy behavior — substituting "请求上游地址失败" —
// hid the real upstream cause from Claude API callers.
func TestClaudeErrorWrapper_PreservesUpstreamMessage(t *testing.T) {
	upstream := errors.New("Post \"https://api.anthropic.com/v1/messages\": dial tcp 1.2.3.4:443: i/o timeout")
	got := ClaudeErrorWrapper(upstream, "test_code", http.StatusBadGateway)
	require.NotNil(t, got)

	msg := got.Error.Message
	assert.NotEqual(t, "请求上游地址失败", msg,
		"must not replace upstream message with synthetic string")
	assert.Contains(t, msg, "i/o timeout",
		"caller must see the actual upstream failure mode")
	assert.NotContains(t, msg, "api.anthropic.com",
		"URL host should be masked by MaskSensitiveInfo")
	assert.Equal(t, http.StatusBadGateway, got.StatusCode)
}

// TestClaudeErrorWrapper_NonNetworkErrorPassesThrough verifies the
// pre-existing escape hatch — errors starting with "get file base64 from url"
// don't get the mask treatment at all (per the original branch logic).
func TestClaudeErrorWrapper_NonNetworkErrorPassesThrough(t *testing.T) {
	upstream := errors.New("validation failed: temperature must be in [0,2]")
	got := ClaudeErrorWrapper(upstream, "test_code", http.StatusBadRequest)
	require.NotNil(t, got)
	assert.Equal(t, "validation failed: temperature must be in [0,2]", got.Error.Message,
		"non-network errors must be passed through verbatim")
}

// TestRelayErrorHandler_IncludesUpstreamBodyOnParseFailure guards Rule 31's
// "showBodyWhenFail=false destroys upstream body" trap. When the upstream
// response is non-JSON (or doesn't fit dto.GeneralErrorResponse), the caller
// must still see what the upstream said in the surfaced error message.
func TestRelayErrorHandler_IncludesUpstreamBodyOnParseFailure(t *testing.T) {
	upstreamBody := `<!DOCTYPE html><html><body><h1>503 Service Unavailable</h1>` +
		`<p>upstream provider temporarily down — retry after 30s</p></body></html>`
	resp := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Body:       io.NopCloser(bytes.NewReader([]byte(upstreamBody))),
		Header:     http.Header{},
	}

	// showBodyWhenFail=false matches all 11 prod call sites.
	apiErr := RelayErrorHandler(context.Background(), resp, false)
	require.NotNil(t, apiErr)
	require.NotNil(t, apiErr.Err)

	assert.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
	got := apiErr.Err.Error()
	assert.Contains(t, got, "503", "status code should be in surfaced error")
	assert.Contains(t, got, "upstream provider temporarily down",
		"upstream body must reach caller, not just server log")
	assert.Contains(t, got, "retry after 30s",
		"actionable text from upstream must survive parse failure")
}

// TestRelayErrorHandler_TruncatesOversizeUpstreamBody guards that we don't
// accidentally pipe a multi-megabyte upstream blob into a JSON error envelope.
func TestRelayErrorHandler_TruncatesOversizeUpstreamBody(t *testing.T) {
	big := strings.Repeat("A", 4096)
	resp := &http.Response{
		StatusCode: http.StatusBadGateway,
		Body:       io.NopCloser(bytes.NewReader([]byte(big))),
		Header:     http.Header{},
	}
	apiErr := RelayErrorHandler(context.Background(), resp, false)
	require.NotNil(t, apiErr)
	require.NotNil(t, apiErr.Err)

	got := apiErr.Err.Error()
	assert.Contains(t, got, "(truncated)",
		"oversize bodies must be truncated with a visible marker")
	assert.Less(t, len(got), len(big), "surfaced error shorter than raw body")
}

// TestTruncateString covers the local helper used by RelayErrorHandler.
func TestTruncateString(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "abc", truncateString("abc", 10))
	assert.Equal(t, "abc", truncateString("abc", 3))
	assert.Equal(t, "abc...(truncated)", truncateString("abcdef", 3))
}
