package remotedesktop

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	errRDPClientClosed = errors.New("rdp stream client closed")

	errRDPClientStalled = errors.New("rdp stream client stalled")
)

type streamClient struct {
	stream  Stream
	session *Session
	ctx     context.Context

	acks chan rdpAck

	done      chan struct{}
	closeOnce sync.Once

	needsKeyFrame bool
}

func newStreamClient(ctx context.Context, stream Stream, session *Session) *streamClient {
	return &streamClient{
		stream:  stream,
		session: session,
		ctx:     ctx,
		acks:    make(chan rdpAck, 8),
		done:    make(chan struct{}),
	}
}

func (c *streamClient) fail() {
	c.closeOnce.Do(func() { close(c.done) })
}

func (c *streamClient) signalAck(ack rdpAck) {
	select {
	case c.acks <- ack:
		return
	default:
	}
	select {
	case <-c.acks:
	default:
	}
	select {
	case c.acks <- ack:
	default:
	}
}

func (c *streamClient) send(frame []byte) error {
	return c.stream.Send(frame)
}

func (c *streamClient) waitAck(batchID uint64, deadline time.Time) (rdpAck, error) {
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	for {
		select {
		case ack := <-c.acks:
			if ack.batchID != batchID {

				continue
			}
			return ack, nil
		case <-timer.C:
			c.session.streams.recordSlowClient()
			return rdpAck{}, errRDPClientStalled
		case <-c.done:
			return rdpAck{}, errRDPClientClosed
		case <-c.ctx.Done():
			return rdpAck{}, c.ctx.Err()
		}
	}
}

type updateHub struct {
	session *Session
	options StreamOptions

	mu      sync.Mutex
	clients []*streamClient
	running bool
	batchID uint64

	kick chan struct{}
}

func newUpdateHub(session *Session, options StreamOptions) *updateHub {
	return &updateHub{session: session, options: options, kick: make(chan struct{}, 1)}
}

func (h *updateHub) currentBatch() uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.batchID
}

func (h *updateHub) attach(client *streamClient) {
	h.mu.Lock()
	client.needsKeyFrame = true
	h.clients = append(h.clients, client)
	running := h.running
	if !running {
		h.running = true
	}
	h.mu.Unlock()

	select {
	case h.kick <- struct{}{}:
	default:
	}
	if !running {
		go h.run()
	}
}

func (h *updateHub) detach(client *streamClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for index, item := range h.clients {
		if item == client {
			h.clients = append(h.clients[:index], h.clients[index+1:]...)
			return
		}
	}
}

func (h *updateHub) clientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

func (h *updateHub) snapshotClients() []*streamClient {
	h.mu.Lock()
	defer h.mu.Unlock()
	clients := make([]*streamClient, len(h.clients))
	copy(clients, h.clients)
	return clients
}

func (h *updateHub) run() {
	defer func() {
		h.mu.Lock()
		h.running = false
		remaining := len(h.clients)
		if remaining > 0 {

			h.running = true
		}
		h.mu.Unlock()
		if remaining > 0 {
			go h.run()
		}
	}()
	ctx := h.session.ctx
	for {
		if ctx.Err() != nil || h.clientCount() == 0 {
			return
		}
		if err := h.step(ctx); err != nil {
			return
		}
	}
}

func (h *updateHub) step(ctx context.Context) error {

	idle := time.NewTimer(h.options.IdleWait)
	defer idle.Stop()
	select {
	case <-h.session.framebuffer.Activity():
	case <-h.kick:
	case <-idle.C:
	case <-ctx.Done():
		return ctx.Err()
	}
	if h.clientCount() == 0 {
		return nil
	}

	if h.clientAwaitsKeyFrame() {
		h.session.framebuffer.ForceKeyFrame()
	}

	merge := time.NewTimer(h.options.MergeWindow)
	select {
	case <-merge.C:
	case <-ctx.Done():
		return ctx.Err()
	}

	snapshotStarted := time.Now()
	update := h.session.framebuffer.TakeUpdate()
	if update.Empty() {
		return nil
	}
	h.session.streams.recordSnapshot(len(update.Rects), time.Since(snapshotStarted))

	encodeStarted := time.Now()
	rects, err := encodeCanvasUpdate(&update)
	if err != nil {

		update.Release()
		h.session.streams.recordEncodeFailure()
		return nil
	}
	payloadBytes := 0
	for _, rect := range rects {
		payloadBytes += len(rect.Payload)
	}
	h.session.streams.recordEncode(len(rects), payloadBytes, time.Since(encodeStarted))

	h.mu.Lock()
	h.batchID++
	batchID := h.batchID
	h.mu.Unlock()

	frame, err := buildRDPUpdateFrame(batchID, &update, rects)
	keyFrame := update.KeyFrame

	update.Release()
	if err != nil {
		h.session.streams.recordEncodeFailure()
		return nil
	}

	h.session.streams.recordBatchSent(keyFrame, len(rects), len(frame), time.Since(snapshotStarted))
	h.deliverBatch(frame, batchID, keyFrame)
	return nil
}

func (h *updateHub) clientAwaitsKeyFrame() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, client := range h.clients {
		if client.needsKeyFrame {
			return true
		}
	}
	return false
}

func (h *updateHub) deliverBatch(frame []byte, batchID uint64, keyFrame bool) {
	clients := h.snapshotClients()

	targets := make([]*streamClient, 0, len(clients))
	for _, client := range clients {
		if client.awaitsKeyFrame() && !keyFrame {

			continue
		}
		if err := client.send(frame); err != nil {

			client.fail()
			continue
		}
		targets = append(targets, client)
	}

	deadline := time.Now().Add(h.options.StallTimeout)
	delivered := 0
	var renderMicros uint32
	for _, client := range targets {
		ack, err := client.waitAck(batchID, deadline)
		if err != nil {
			client.fail()
			continue
		}
		delivered++
		if ack.renderMicros > renderMicros {
			renderMicros = ack.renderMicros
		}
		if keyFrame {
			h.markKeyFrameDelivered(client)
		}
	}
	h.session.streams.recordAck(delivered > 0, float64(renderMicros)/1000, h.session.framebuffer.Updates())
}

func (c *streamClient) awaitsKeyFrame() bool {
	c.session.hub.mu.Lock()
	defer c.session.hub.mu.Unlock()
	return c.needsKeyFrame
}

func (h *updateHub) markKeyFrameDelivered(client *streamClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	client.needsKeyFrame = false
}
