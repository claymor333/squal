package ai

import "context"

// nativeTransport is the OpenAI native-tools transport over Client.CompleteTools.
type nativeTransport struct {
	client *Client
}

func newNativeTransport(c *Client) *nativeTransport {
	return &nativeTransport{client: c}
}

func (t *nativeTransport) Complete(ctx context.Context, msgs []Message, tools []ToolDef) (Response, error) {
	return t.client.CompleteTools(ctx, msgs, tools)
}

func (t *nativeTransport) Name() string { return "native" }
