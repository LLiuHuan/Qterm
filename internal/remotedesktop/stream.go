package remotedesktop

import (
	"context"
	"time"
)

const RDPStreamName = "rdp"

const RDPProtocolVersion = rdpProtocolVersion

type Stream interface {
	Context() context.Context
	Send([]byte) error
	Receive() ([]byte, error)
	Close() error
}

type TokenVerifier interface {
	Consume(sessionID, token string) bool
}

type SessionResolver interface {
	StreamSession(id string) (*Session, bool)
}

type StreamOptions struct {
	MergeWindow time.Duration

	IdleWait time.Duration

	StallTimeout time.Duration
}

func DefaultStreamOptions() StreamOptions {
	return StreamOptions{
		MergeWindow:  12 * time.Millisecond,
		IdleWait:     500 * time.Millisecond,
		StallTimeout: 5 * time.Second,
	}
}

func ServeStream(stream Stream, resolver SessionResolver, tokens TokenVerifier, options StreamOptions) {
	if stream == nil || resolver == nil || tokens == nil {
		return
	}
	defer stream.Close()

	bindFrame, err := stream.Receive()
	if err != nil {
		return
	}
	bind, err := parseRDPBindFrame(bindFrame)
	if err != nil {
		_ = stream.Send(encodeRDPErrorFrame(rdpErrorProtocol, "画面流绑定帧无效"))
		return
	}
	if !tokens.Consume(bind.sessionID, bind.token) {
		_ = stream.Send(encodeRDPErrorFrame(rdpErrorBinding, "画面流绑定令牌无效或已过期"))
		return
	}
	session, ok := resolver.StreamSession(bind.sessionID)
	if !ok || session == nil || session.framebuffer == nil {
		_ = stream.Send(encodeRDPErrorFrame(rdpErrorSession, "远程桌面会话不存在或已结束"))
		return
	}

	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	client := newStreamClient(ctx, stream, session)
	session.streams.addClient(1)
	defer session.streams.addClient(-1)

	width, height := session.framebuffer.Size()
	if err := stream.Send(encodeRDPReadyFrame(width, height, session.hub.currentBatch())); err != nil {
		return
	}

	results := make(chan error, 2)
	go func() { results <- client.readLoop(ctx) }()

	session.hub.attach(client)

	select {
	case <-results:
	case <-ctx.Done():
	case <-session.done:
	case <-client.done:

	}
	cancel()
	session.hub.detach(client)
	client.fail()
}

func (c *streamClient) readLoop(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		frame, err := c.stream.Receive()
		if err != nil {
			return err
		}
		op, ack, drop, err := parseRDPControlFrame(frame)
		if err != nil {

			return err
		}
		switch op {
		case rdpOpAck:
			c.signalAck(ack)
		case rdpOpDrop:

			c.session.framebuffer.ForceKeyFrame()
			c.session.streams.recordDecodeDrop()
			c.signalAck(rdpAck{batchID: drop.batchID})
		}
	}
}
