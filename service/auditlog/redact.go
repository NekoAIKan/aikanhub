package auditlog

import (
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const redactedMarker = "***REDACTED***"

// bearerRe catches `Bearer sk-...` / `Bearer eyJ...` style secrets that show
// up inside non-JSON bodies (form posts, proxied raw payloads) where the
// structured walk can't reach them.
var bearerRe = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._\-]+`)

// redactBody returns a privacy-safe copy of body. For JSON it deep-walks
// and masks any object key whose lower-cased name is in keys (recursing
// into nested objects/arrays). For anything else it falls back to a regex
// sweep for bearer tokens. It never returns an error: redaction must not be
// able to fail the audit write — worst case it returns the regex-swept
// bytes and the caller flags redacted=true regardless.
//
// keys are matched case-insensitively; pass the audit_setting.RedactKeys
// slice (already lower-case by convention, but we lower-case here too so a
// mis-cased operator entry still works).
func redactBody(body []byte, keys []string) []byte {
	if len(body) == 0 {
		return body
	}
	keySet := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		keySet[strings.ToLower(strings.TrimSpace(k))] = struct{}{}
	}

	var v any
	if err := common.Unmarshal(body, &v); err == nil {
		redacted := redactValue(v, keySet)
		if out, mErr := common.Marshal(redacted); mErr == nil {
			return out
		}
		// Marshal of a redacted tree should never fail; if it somehow
		// does, fall through to the regex sweep rather than leak raw JSON.
	}
	return bearerRe.ReplaceAll(body, []byte("Bearer "+redactedMarker))
}

// redactValue recursively masks matching keys. Maps are rebuilt (so we
// don't mutate the caller's decoded tree); slices are walked element-wise.
func redactValue(v any, keySet map[string]struct{}) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if _, hit := keySet[strings.ToLower(k)]; hit {
				out[k] = redactedMarker
				continue
			}
			out[k] = redactValue(val, keySet)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = redactValue(e, keySet)
		}
		return out
	default:
		return v
	}
}
