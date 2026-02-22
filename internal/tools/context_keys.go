package tools

import "context"

// Tool execution context keys.
// These replace mutable setter fields on tool instances, making tools thread-safe
// for concurrent execution. Values are injected into context by the registry
// and read by individual tools during Execute().

type toolContextKey string

const (
	ctxChannel    toolContextKey = "tool_channel"
	ctxChatID     toolContextKey = "tool_chat_id"
	ctxPeerKind   toolContextKey = "tool_peer_kind"
	ctxSandboxKey toolContextKey = "tool_sandbox_key"
	ctxAsyncCB    toolContextKey = "tool_async_cb"
)

func WithToolChannel(ctx context.Context, channel string) context.Context {
	return context.WithValue(ctx, ctxChannel, channel)
}

func ToolChannelFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxChannel).(string)
	return v
}

func WithToolChatID(ctx context.Context, chatID string) context.Context {
	return context.WithValue(ctx, ctxChatID, chatID)
}

func ToolChatIDFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxChatID).(string)
	return v
}

func WithToolPeerKind(ctx context.Context, peerKind string) context.Context {
	return context.WithValue(ctx, ctxPeerKind, peerKind)
}

func ToolPeerKindFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxPeerKind).(string)
	return v
}

func WithToolSandboxKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, ctxSandboxKey, key)
}

func ToolSandboxKeyFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxSandboxKey).(string)
	return v
}

func WithToolAsyncCB(ctx context.Context, cb AsyncCallback) context.Context {
	return context.WithValue(ctx, ctxAsyncCB, cb)
}

func ToolAsyncCBFromCtx(ctx context.Context) AsyncCallback {
	v, _ := ctx.Value(ctxAsyncCB).(AsyncCallback)
	return v
}
