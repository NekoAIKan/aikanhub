// materialize.go — turn whatever the user gave us into a public HTTPS
// URL that ARK CreateAsset can fetch. http(s) URLs pass through; data
// URIs and bare base64 strings get uploaded to Tencent COS first.
package imageaudit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// materializeURL ensures the audit pipeline has a public HTTPS URL.
// For URL inputs we just return them. For base64/data URIs we upload
// to COS and return the public URL.
func materializeURL(ctx context.Context, cfg Config, src string) (string, error) {
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		return src, nil
	}
	if !cfg.HasCOS() {
		return "", errors.New("imageaudit: non-URL source requires COS_BUCKET / COS_SECRET_ID / COS_SECRET_KEY")
	}
	raw, contentType, err := decodeDataURI(src)
	if err != nil {
		return "", fmt.Errorf("decode data URI: %w", err)
	}
	if len(raw) == 0 {
		return "", errors.New("decoded image is empty")
	}
	objectKey := buildObjectKey(cfg.COSPrefix, contentType)
	publicURL, err := uploadToCOS(ctx, cfg, objectKey, raw, contentType)
	if err != nil {
		return "", fmt.Errorf("cos upload: %w", err)
	}
	return publicURL, nil
}

// buildObjectKey produces a unique, sortable key like
// "<prefix>/2026/05/07/<nanosec>-<6hex>.jpg". Predictable enough for
// log greps, unique enough that collisions are statistically zero.
func buildObjectKey(prefix, contentType string) string {
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		prefix = "aikanhub-audit"
	}
	now := time.Now().UTC()
	day := now.Format("2006/01/02")
	rb := make([]byte, 6)
	_, _ = rand.Read(rb)
	suffix := hex.EncodeToString(rb)
	return fmt.Sprintf("%s/%s/%d-%s%s", prefix, day, now.UnixNano(), suffix, extOf(contentType))
}
