package terminal

import (
	"context"
	"strings"
	"sync"
)

type writerRequest struct {
	ctx  context.Context
	data []byte
	done chan error
}

type sessionQueue struct {
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	retired chan struct{}
	queue   chan writerRequest
	write   SessionWriteFunc
	id      string
	done    chan struct{}
	fail    func(string, error)
}

func (q *sessionQueue) run(previous *sessionQueue, finished func()) {
	defer finished()
	defer q.cancel()
	defer close(q.done)
	defer func() {

		for {
			select {
			case request := <-q.queue:
				request.done <- ErrSessionNotEligible
			default:
				return
			}
		}
	}()

	if previous != nil {

		<-previous.done
	}

	for {
		select {
		case <-q.ctx.Done():
			return
		case <-q.retired:
			return
		case request := <-q.queue:
			select {
			case <-q.retired:
				request.done <- ErrSessionNotEligible
				return
			default:
			}
			if err := request.ctx.Err(); err != nil {
				request.done <- err
				continue
			}
			if err := q.ctx.Err(); err != nil {
				request.done <- err
				return
			}

			writeCtx, cancel := context.WithTimeout(q.ctx, writeTimeout)
			err := q.write(writeCtx, q.id, request.data)
			cancel()
			if err != nil && q.fail != nil {
				q.fail(q.id, err)
			}
			request.done <- err
		}
	}
}

func (q *sessionQueue) submit(ctx context.Context, data []byte) (<-chan error, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	select {
	case <-q.retired:
		return nil, ErrSessionNotEligible
	case <-q.ctx.Done():
		return nil, ErrControlNotReady
	default:
	}
	request := writerRequest{ctx: ctx, data: append([]byte(nil), data...), done: make(chan error, 1)}
	select {
	case q.queue <- request:
		return request.done, nil
	default:
		return nil, ErrWriterQueueFull
	}
}

func (q *sessionQueue) retire() {
	q.mu.Lock()
	defer q.mu.Unlock()
	select {
	case <-q.retired:
	default:
		close(q.retired)
	}
}

type sessionWriter struct {
	mu     sync.Mutex
	root   context.Context
	write  SessionWriteFunc
	fail   func(string, error)
	queues map[string]*sessionQueue
	all    map[*sessionQueue]struct{}
	tails  map[string]*sessionQueue
	wg     sync.WaitGroup
	closed bool
}

func newSessionWriter(parent context.Context, write SessionWriteFunc, fail func(string, error)) *sessionWriter {
	return &sessionWriter{root: parent, write: write, fail: fail, queues: make(map[string]*sessionQueue), all: make(map[*sessionQueue]struct{}), tails: make(map[string]*sessionQueue)}
}

func (w *sessionWriter) queueFor(id string) (*sessionQueue, error) {
	if strings.TrimSpace(id) == "" {
		return nil, ErrSessionRequired
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || w.write == nil {
		return nil, ErrControlNotReady
	}
	if q := w.queues[id]; q != nil {
		return q, nil
	}
	ctx, cancel := context.WithCancel(w.root)
	q := &sessionQueue{ctx: ctx, cancel: cancel, retired: make(chan struct{}), queue: make(chan writerRequest, maxWriterQueue), write: w.write, id: id, done: make(chan struct{}), fail: w.fail}
	previous := w.tails[id]
	w.tails[id] = q
	w.queues[id] = q
	w.all[q] = struct{}{}
	w.wg.Add(1)
	go q.run(previous, func() {
		w.mu.Lock()
		delete(w.all, q)
		if w.tails[id] == q {
			delete(w.tails, id)
		}
		if w.queues[id] == q {
			delete(w.queues, id)
		}
		w.mu.Unlock()
		w.wg.Done()
	})
	return q, nil
}

func (w *sessionWriter) submit(ctx context.Context, id string, data []byte) (<-chan error, error) {
	q, err := w.queueFor(id)
	if err != nil {
		return nil, err
	}
	return q.submit(ctx, data)
}

func (w *sessionWriter) enqueue(ctx context.Context, id string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	_, err := w.submit(context.WithoutCancel(ctx), id, data)
	return err
}

func (w *sessionWriter) writeWait(ctx context.Context, id string, data []byte) error {
	done, err := w.submit(ctx, id, data)
	if err != nil {
		return err
	}
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *sessionWriter) retire(id string) {
	w.mu.Lock()
	q := w.queues[id]
	delete(w.queues, id)
	w.mu.Unlock()
	if q != nil {
		q.retire()
	}
}

func (w *sessionWriter) close() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		w.wg.Wait()
		return
	}
	w.closed = true
	for q := range w.all {
		q.cancel()
	}
	w.queues = make(map[string]*sessionQueue)
	w.mu.Unlock()
	w.wg.Wait()
}
