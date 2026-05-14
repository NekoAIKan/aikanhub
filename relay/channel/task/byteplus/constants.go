package byteplus

// ModelList is the suggested default model list for the BytePlus Seedance
// channel — the overseas-only Seedance 2.0 series. The `-global` suffix
// distinguishes these from the China-side Doubao Seedance models served
// by the doubao adaptor.
//
// Upstream BytePlus expects the long-form model IDs (e.g.
// `seedance-2-0-260128`) or the operator-provisioned endpoint IDs
// (`ep-...`); operators rewrite client-facing names to upstream names via
// the channel's model-mapping field.
var ModelList = []string{
	"seedance-2.0-global",
	"seedance-2.0-fast-global",
}

var ChannelName = "byteplus-video"
