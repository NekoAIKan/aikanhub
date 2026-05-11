package imageaudit

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// Volcano ARK Assets API — uses Volcengine V4 signing (the same scheme as
// e2e_audit_api_test.py in this repo's parent dir).
//
// Endpoints used:
//   POST /  ?Action=CreateAssetGroup  Version=2024-01-01
//   POST /  ?Action=ListAssetGroups   Version=2024-01-01
//   POST /  ?Action=CreateAsset       Version=2024-01-01
//   POST /  ?Action=GetAsset          Version=2024-01-01
//
// Request bodies are JSON; responses are {ResponseMetadata:{}, Result:{}}.

const (
	arkHost           = "ark.cn-beijing.volcengineapi.com"
	arkRegion         = "cn-beijing"
	arkService        = "ark"
	arkVersion        = "2024-01-01"
	arkContentType    = "application/json"
	arkSignedHeaders  = "content-type;host;x-content-sha256;x-date"
	arkRequestTimeout = 30 * time.Second
)

// arkResponse is the common envelope for every Assets call.
type arkResponse struct {
	ResponseMetadata struct {
		RequestId string `json:"RequestId"`
		Action    string `json:"Action"`
		Region    string `json:"Region"`
		Service   string `json:"Service"`
		Version   string `json:"Version"`
		Error     *struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error,omitempty"`
	} `json:"ResponseMetadata"`
	Result json.RawMessage `json:"Result"`
}

// arkClient wraps the AK/SK + ProjectName so call sites don't repeat them.
type arkClient struct {
	cfg Config
	hc  *http.Client
}

func newARKClient(cfg Config) *arkClient {
	return &arkClient{
		cfg: cfg,
		hc:  &http.Client{Timeout: arkRequestTimeout},
	}
}

// callAction issues a signed POST to the ARK control-plane and unmarshals the
// `Result` field into out. Service-level errors (returned as a populated
// ResponseMetadata.Error) come back as Go errors.
func (c *arkClient) callAction(ctx context.Context, action string, body, out any) error {
	if !c.cfg.HasARK() {
		return fmt.Errorf("ARK credentials missing (ARK_AK / ARK_SK)")
	}
	// Inject ProjectName so callers don't have to remember.
	bodyMap, err := toMap(body)
	if err != nil {
		return fmt.Errorf("ark.%s: marshal body: %w", action, err)
	}
	if _, ok := bodyMap["ProjectName"]; !ok && c.cfg.ARKProject != "" {
		bodyMap["ProjectName"] = c.cfg.ARKProject
	}
	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return fmt.Errorf("ark.%s: marshal merged body: %w", action, err)
	}
	bodySHA := sha256Hex(bodyBytes)

	q := url.Values{}
	q.Set("Action", action)
	q.Set("Version", arkVersion)
	headers := signV4(
		"POST", arkHost, "/", q,
		bodySHA,
		c.cfg.ARKAk, c.cfg.ARKSk,
		arkService, arkRegion,
		arkContentType,
	)

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://"+arkHost+"/?"+q.Encode(), bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("ark.%s: new request: %w", action, err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("ark.%s: do: %w", action, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("ark.%s: read body: %w", action, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ark.%s: HTTP %d: %s", action, resp.StatusCode, truncate(string(respBody), 500))
	}

	var env arkResponse
	if err := json.Unmarshal(respBody, &env); err != nil {
		return fmt.Errorf("ark.%s: unmarshal envelope: %w; body=%s", action, err, truncate(string(respBody), 500))
	}
	if env.ResponseMetadata.Error != nil {
		return fmt.Errorf("ark.%s: %s — %s",
			action,
			env.ResponseMetadata.Error.Code,
			env.ResponseMetadata.Error.Message,
		)
	}
	if out == nil || len(env.Result) == 0 {
		return nil
	}
	if err := json.Unmarshal(env.Result, out); err != nil {
		return fmt.Errorf("ark.%s: unmarshal Result: %w; result=%s", action, err, truncate(string(env.Result), 500))
	}
	return nil
}

// ---------------------------------------------------------------------------
// CreateAssetGroup / ListAssetGroups — used to obtain a stable group id.
// ---------------------------------------------------------------------------

type arkGroup struct {
	Id          string `json:"Id"`
	Name        string `json:"Name"`
	GroupType   string `json:"GroupType"`
	Description string `json:"Description"`
}

func (c *arkClient) createAssetGroup(ctx context.Context, name string) (string, error) {
	body := map[string]any{
		"Name":        name,
		"GroupType":   "AIGC",
		"Description": name,
	}
	var g arkGroup
	if err := c.callAction(ctx, "CreateAssetGroup", body, &g); err != nil {
		return "", err
	}
	if g.Id == "" {
		return "", fmt.Errorf("CreateAssetGroup: empty Id in result")
	}
	return g.Id, nil
}

type listAssetGroupsResult struct {
	Items []arkGroup `json:"Items"`
	Total int        `json:"Total"`
}

func (c *arkClient) findAssetGroupByName(ctx context.Context, name string) (string, error) {
	body := map[string]any{
		"Filter":     map[string]any{"GroupType": "AIGC", "Name": name},
		"PageNumber": 1,
		"PageSize":   20,
		"SortBy":     "CreateTime",
		"SortOrder":  "Desc",
	}
	var res listAssetGroupsResult
	if err := c.callAction(ctx, "ListAssetGroups", body, &res); err != nil {
		return "", err
	}
	for _, g := range res.Items {
		if g.Name == name {
			return g.Id, nil
		}
	}
	return "", nil
}

// ensureAssetGroup returns the configured GROUP_ID when set, else find-or-
// create by GROUP_NAME. The result is cached on the client for the process'
// lifetime so we only do the lookup once.
func (c *arkClient) ensureAssetGroup(ctx context.Context) (string, error) {
	if c.cfg.ARKGroupID != "" {
		return c.cfg.ARKGroupID, nil
	}
	if c.cfg.ARKGroupName == "" {
		return "", fmt.Errorf("either ARK_ASSETS_GROUP_ID or ARK_ASSETS_GROUP_NAME must be set")
	}
	if id, err := c.findAssetGroupByName(ctx, c.cfg.ARKGroupName); err != nil {
		// Listing failed — surface the error rather than silently creating a duplicate.
		return "", fmt.Errorf("findAssetGroupByName: %w", err)
	} else if id != "" {
		return id, nil
	}
	common.SysLog(fmt.Sprintf("imageaudit: creating new ARK asset group name=%s project=%s",
		c.cfg.ARKGroupName, c.cfg.ARKProject))
	return c.createAssetGroup(ctx, c.cfg.ARKGroupName)
}

// ---------------------------------------------------------------------------
// CreateAsset / GetAsset — submit URL → poll → asset:// URI.
// ---------------------------------------------------------------------------

type arkAssetResult struct {
	Id     string          `json:"Id"`
	Status string          `json:"Status"`
	URL    string          `json:"URL"`
	Error  json.RawMessage `json:"Error,omitempty"`
}

func (c *arkClient) createAsset(ctx context.Context, groupId, imgURL string) (string, error) {
	body := map[string]any{
		"GroupId":   groupId,
		"URL":       imgURL,
		"AssetType": "Image",
	}
	var res arkAssetResult
	if err := c.callAction(ctx, "CreateAsset", body, &res); err != nil {
		return "", err
	}
	if res.Id == "" {
		return "", fmt.Errorf("CreateAsset: empty Id in result")
	}
	return res.Id, nil
}

func (c *arkClient) getAsset(ctx context.Context, id string) (*arkAssetResult, error) {
	body := map[string]any{"Id": id}
	var res arkAssetResult
	if err := c.callAction(ctx, "GetAsset", body, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// AuditError wraps an audit-stage failure so callers can distinguish it from
// transient infra errors. Status carries the upstream Status string ("Failed",
// "Processing", ...) and Reason carries Volcano's explanation when available.
type AuditError struct {
	AssetId string
	Status  string
	Reason  string
}

func (e *AuditError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("image audit %s (asset=%s): %s", e.Status, e.AssetId, e.Reason)
	}
	return fmt.Sprintf("image audit %s (asset=%s)", e.Status, e.AssetId)
}

// ---------------------------------------------------------------------------
// Volcengine V4 signing (same algorithm as the Python e2e script).
// ---------------------------------------------------------------------------

func signV4(method, host, path string, query url.Values, bodySHA256, ak, sk, service, region, contentType string) map[string]string {
	now := time.Now().UTC()
	xDate := now.Format("20060102T150405Z")
	shortDate := xDate[:8]

	canonical := strings.Join([]string{
		strings.ToUpper(method),
		path,
		canonicalQueryString(query),
		"content-type:" + contentType,
		"host:" + host,
		"x-content-sha256:" + bodySHA256,
		"x-date:" + xDate,
		"",
		arkSignedHeaders,
		bodySHA256,
	}, "\n")
	credScope := shortDate + "/" + region + "/" + service + "/request"
	stringToSign := strings.Join([]string{
		"HMAC-SHA256",
		xDate,
		credScope,
		sha256Hex([]byte(canonical)),
	}, "\n")

	k := hmacSHA256([]byte(sk), shortDate)
	k = hmacSHA256(k, region)
	k = hmacSHA256(k, service)
	k = hmacSHA256(k, "request")
	signature := hex.EncodeToString(hmacSHA256(k, stringToSign))

	return map[string]string{
		"Host":             host,
		"Content-Type":     contentType,
		"X-Date":           xDate,
		"X-Content-Sha256": bodySHA256,
		"Authorization": fmt.Sprintf(
			"HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
			ak, credScope, arkSignedHeaders, signature,
		),
	}
}

func canonicalQueryString(q url.Values) string {
	if len(q) == 0 {
		return ""
	}
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(q))
	for _, k := range keys {
		for _, v := range q[k] {
			parts = append(parts,
				volcanoQueryEscape(k)+"="+volcanoQueryEscape(v))
		}
	}
	return strings.Join(parts, "&")
}

// volcanoQueryEscape mirrors the Python script's `quote(s, safe="-_.~")`.
// url.QueryEscape escapes "+" oddly for our purposes — re-do it.
func volcanoQueryEscape(s string) string {
	enc := url.QueryEscape(s)
	enc = strings.ReplaceAll(enc, "+", "%20")
	return enc
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func toMap(v any) (map[string]any, error) {
	if v == nil {
		return map[string]any{}, nil
	}
	if m, ok := v.(map[string]any); ok {
		// Copy so we don't mutate the caller's map when ProjectName is injected.
		out := make(map[string]any, len(m)+1)
		for k, vv := range m {
			out[k] = vv
		}
		return out, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	out := make(map[string]any)
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
