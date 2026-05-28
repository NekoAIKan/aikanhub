// ark_test.go — unit tests for callAction's error classification.
//
// The full ARK round-trip is covered by the container e2e test; here we
// only verify that an HTTP 4xx response from the control plane comes back
// as a typed *ArkUpstreamError with the upstream's Message intact, so the
// relay/controller layer can forward it as a 4xx to the gateway client.
package imageaudit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reproduces the literal upstream payload from the SeedVR2 super-resolution
// reference-image bug — width > 6000px is the exact case that motivated the
// typed-error rework. Region/Action/Code/Message copied verbatim.
const widthTooLargeBody = `{"ResponseMetadata":{"RequestId":"202605282340389F859187A1D355E500AD","Action":"CreateAsset","Version":"2024-01-01","Service":"ark","Region":"ap-southeast-1","Error":{"Code":"InvalidParameter.WidthTooLarge","Message":"Width must be between 300px and 6000px.","Data":null}}}`

// newTestArkClient wires callAction's hardcoded https://<host>/ URL through
// the httptest.NewTLSServer's self-signed certificate. We don't care about
// signature verification on the receiving side; the server returns whatever
// status/body the test specifies.
func newTestArkClient(t *testing.T, srv *httptest.Server) *arkClient {
	t.Helper()
	host := strings.TrimPrefix(srv.URL, "https://")
	return &arkClient{
		resolved: Resolved{
			Ak:      "test-ak",
			Sk:      "test-sk",
			Host:    host,
			Region:  "ap-southeast-1",
			Service: "ark",
			Project: "tianwenyue-6",
		},
		hc: srv.Client(),
	}
}

func TestCallAction_HTTP400_ReturnsTypedUpstreamError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(widthTooLargeBody))
	}))
	defer srv.Close()

	c := newTestArkClient(t, srv)
	err := c.callAction(context.Background(), "CreateAsset",
		map[string]any{"GroupId": "g1", "URL": "https://example.com/x.jpg", "AssetType": "Image"}, nil)
	require.Error(t, err)

	var ark *ArkUpstreamError
	require.True(t, errors.As(err, &ark), "expected *ArkUpstreamError; got %T: %v", err, err)
	assert.Equal(t, http.StatusBadRequest, ark.HTTPStatus)
	assert.Equal(t, "CreateAsset", ark.Action)
	assert.Equal(t, "InvalidParameter.WidthTooLarge", ark.MetaCode)
	assert.Equal(t, "Width must be between 300px and 6000px.", ark.MetaMsg)
	assert.True(t, ark.IsClientFault(), "HTTP 400 must be classified as client fault")
	assert.Contains(t, ark.RawBody, "WidthTooLarge", "raw body retained for diagnostics")
}

func TestCallAction_HTTP500_NotClientFault(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"ResponseMetadata":{"Error":{"Code":"InternalError","Message":"boom"}}}`))
	}))
	defer srv.Close()

	c := newTestArkClient(t, srv)
	err := c.callAction(context.Background(), "CreateAsset", map[string]any{}, nil)
	require.Error(t, err)

	var ark *ArkUpstreamError
	require.True(t, errors.As(err, &ark))
	assert.Equal(t, http.StatusInternalServerError, ark.HTTPStatus)
	assert.False(t, ark.IsClientFault(), "HTTP 5xx must NOT be classified as client fault")
}

func TestCallAction_HTTP200WithEnvelopeError_ReturnsTypedUpstreamError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ResponseMetadata":{"Error":{"Code":"SomeCode","Message":"some msg"}}}`))
	}))
	defer srv.Close()

	c := newTestArkClient(t, srv)
	err := c.callAction(context.Background(), "CreateAsset", map[string]any{}, nil)
	require.Error(t, err)

	var ark *ArkUpstreamError
	require.True(t, errors.As(err, &ark))
	assert.Equal(t, http.StatusOK, ark.HTTPStatus)
	assert.Equal(t, "SomeCode", ark.MetaCode)
	assert.Equal(t, "some msg", ark.MetaMsg)
	assert.False(t, ark.IsClientFault(), "HTTP 200 with envelope error is NOT client fault")
}

func TestArkUpstreamError_Error_FormatsAllFields(t *testing.T) {
	e := &ArkUpstreamError{
		Action:     "CreateAsset",
		HTTPStatus: 400,
		MetaCode:   "InvalidParameter.WidthTooLarge",
		MetaMsg:    "Width must be between 300px and 6000px.",
	}
	got := e.Error()
	assert.Contains(t, got, "CreateAsset")
	assert.Contains(t, got, "400")
	assert.Contains(t, got, "InvalidParameter.WidthTooLarge")
	assert.Contains(t, got, "Width must be between 300px and 6000px.")
}
