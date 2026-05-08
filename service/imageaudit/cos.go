package imageaudit

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Tencent COS uses its own V5 signing scheme (q-sign-algorithm=sha1) — NOT
// AWS SigV4 — so we implement it manually rather than pulling in the official
// SDK (~1MB). See https://cloud.tencent.com/document/product/436/7778 for the
// full spec.
//
// Algorithm:
//
//	SignKey      = HMAC-SHA1(SecretKey, KeyTime)
//	HttpString   = method "\n" path "\n" canonicalQuery "\n" canonicalHeaders "\n"
//	StringToSign = "sha1" "\n" KeyTime "\n" sha1(HttpString) "\n"
//	Signature    = HMAC-SHA1(SignKey, StringToSign)
//
// Authorization header is a flat URL-style string of fields.

const cosUploadTimeout = 120 * time.Second

// uploadToCOS PUTs a single object and returns its public URL.
//
// objectKey must NOT start with a leading slash; the implementation adds one.
// contentType is sent as-is (Tencent COS preserves it on download).
func uploadToCOS(ctx context.Context, cfg Config, objectKey string, body []byte, contentType string) (string, error) {
	if !cfg.HasCOS() {
		return "", fmt.Errorf("COS not configured (need COS_BUCKET / COS_SECRET_ID / COS_SECRET_KEY)")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	objectKey = strings.TrimLeft(objectKey, "/")
	pathname := "/" + objectKey
	host := cfg.COSEndpoint()

	headers := map[string]string{
		"Host":           host,
		"Content-Type":   contentType,
		"Content-Length": strconv.Itoa(len(body)),
		"x-cos-acl":      "public-read", // ARK CreateAsset must be able to fetch
	}

	auth := signCOS("put", pathname, nil, headers, cfg.COSSecretID, cfg.COSSecretKey, 600)
	headers["Authorization"] = auth

	req, err := http.NewRequestWithContext(ctx, "PUT",
		"https://"+host+pathname, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("cos: new request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	hc := &http.Client{Timeout: cosUploadTimeout}
	resp, err := hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("cos: do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("cos: PUT %s → HTTP %d: %s",
			pathname, resp.StatusCode, truncate(string(respBody), 500))
	}
	return cfg.PublicURL(pathname), nil
}

// signCOS implements Tencent COS V5 signing.
//
// expiresIn controls the q-key-time / q-sign-time window in seconds. PUT
// uploads can use a short window since the request is sent immediately.
func signCOS(method, pathname string, params map[string]string, headers map[string]string,
	secretID, secretKey string, expiresIn int64) string {

	method = strings.ToLower(method)
	now := time.Now().Unix()
	keyTime := fmt.Sprintf("%d;%d", now-60, now+expiresIn)

	// q-sign-key.
	signKey := hexHMAC1([]byte(secretKey), keyTime)

	// q-header-list / q-url-param-list — both lower-cased, sorted, ;-joined.
	hdrKeys, canonicalHeaders := canonicalCOSPairs(headers)
	paramKeys, canonicalParams := canonicalCOSPairs(stringMap(params))

	httpString := strings.Join([]string{
		method,
		pathname,
		canonicalParams,
		canonicalHeaders,
		"",
	}, "\n")
	stringToSign := strings.Join([]string{
		"sha1",
		keyTime,
		sha1Hex([]byte(httpString)),
		"",
	}, "\n")
	signature := hexHMAC1([]byte(signKey), stringToSign)

	parts := []string{
		"q-sign-algorithm=sha1",
		"q-ak=" + secretID,
		"q-sign-time=" + keyTime,
		"q-key-time=" + keyTime,
		"q-header-list=" + strings.Join(hdrKeys, ";"),
		"q-url-param-list=" + strings.Join(paramKeys, ";"),
		"q-signature=" + signature,
	}
	return strings.Join(parts, "&")
}

// canonicalCOSPairs returns (sortedLowercaseKeys, "k1=v1&k2=v2&...") with
// keys lowercased and values percent-encoded per the COS spec (rfc3986
// reserve set with case-insensitive sort).
func canonicalCOSPairs(in map[string]string) ([]string, string) {
	if len(in) == 0 {
		return []string{}, ""
	}
	type kv struct{ k, v string }
	pairs := make([]kv, 0, len(in))
	for k, v := range in {
		pairs = append(pairs, kv{strings.ToLower(k), v})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].k < pairs[j].k })

	keys := make([]string, 0, len(pairs))
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		keys = append(keys, p.k)
		parts = append(parts, cosEscape(p.k)+"="+cosEscape(p.v))
	}
	return keys, strings.Join(parts, "&")
}

// cosEscape is rfc3986 unreserved + "-_.~".
func cosEscape(s string) string {
	// url.QueryEscape produces application/x-www-form-urlencoded encoding,
	// which uses "+" for space — COS expects %20. Patch it.
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func sha1Hex(b []byte) string {
	h := sha1.Sum(b)
	return hex.EncodeToString(h[:])
}

func hexHMAC1(key []byte, data string) string {
	h := hmac.New(sha1.New, key)
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func stringMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

// inferContentType guesses a MIME type from a base64 data URI's metadata, or
// falls back by sniffing the first bytes of decoded content. Used so the
// COS upload presents a sensible Content-Type to ARK.
func inferContentType(dataURI string, decoded []byte) string {
	if strings.HasPrefix(dataURI, "data:") {
		if i := strings.Index(dataURI, ";"); i > 5 {
			return dataURI[5:i]
		}
		if i := strings.Index(dataURI, ","); i > 5 {
			return dataURI[5:i]
		}
	}
	if t := http.DetectContentType(decoded); t != "" && t != "application/octet-stream" {
		return t
	}
	return "image/jpeg"
}

// extOf maps a content-type back to a usable file extension for the COS
// object key. Known image/* values get specific suffixes; everything else
// falls back to ".bin".
func extOf(contentType string) string {
	switch strings.ToLower(contentType) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/bmp":
		return ".bmp"
	case "image/heic":
		return ".heic"
	case "image/heif":
		return ".heif"
	}
	return ".bin"
}

// decodeDataURI unwraps a "data:[<mime>][;base64],<payload>" URI and returns
// the raw bytes plus the inferred MIME type. base64 is the only encoding we
// support — bare data: URIs ("data:,hi") are rare for images and out of scope.
func decodeDataURI(s string) ([]byte, string, error) {
	if !strings.HasPrefix(s, "data:") {
		// Not a data URI — assume a bare base64 string.
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
		if err != nil {
			return nil, "", fmt.Errorf("decode bare base64: %w", err)
		}
		return raw, http.DetectContentType(raw), nil
	}
	comma := strings.Index(s, ",")
	if comma < 0 {
		return nil, "", fmt.Errorf("malformed data URI (no comma)")
	}
	header, payload := s[:comma], s[comma+1:]
	contentType := inferContentType(s, nil)
	if !strings.Contains(header, ";base64") {
		// Plain (URL-encoded) data URI — decode as-is.
		decoded, err := url.QueryUnescape(payload)
		if err != nil {
			return nil, "", fmt.Errorf("decode percent-encoded data URI: %w", err)
		}
		return []byte(decoded), contentType, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(payload))
	if err != nil {
		return nil, "", fmt.Errorf("decode base64 data URI: %w", err)
	}
	if contentType == "" {
		contentType = http.DetectContentType(raw)
	}
	return raw, contentType, nil
}
