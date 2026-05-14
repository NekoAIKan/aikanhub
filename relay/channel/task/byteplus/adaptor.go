// Package byteplus is the overseas (BytePlus ModelArk) sibling of the
// Doubao Seedance video adaptor. The BytePlus and Volcano Ark APIs are
// structurally identical — same `/api/v3/contents/generations/tasks`
// endpoint, same auth, same response shape — they differ only in the
// upstream host (`ark.ap-southeast.bytepluses.com` vs
// `ark.cn-beijing.volces.com`) and the model ID prefix (no `doubao-`).
//
// The host comes from the operator-set channel base URL, so this adaptor
// embeds the Doubao TaskAdaptor and only overrides the model list /
// channel name used for the admin UI auto-fill and log labelling.
package byteplus

import (
	taskdoubao "github.com/QuantumNous/new-api/relay/channel/task/doubao"
)

type TaskAdaptor struct {
	taskdoubao.TaskAdaptor
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}
