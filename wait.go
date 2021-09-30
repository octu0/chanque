package chanque

import (
	"context"
	"sync/atomic"
	"time"
)

type Wait struct {
	ctx     context.Context
	cancel  context.CancelFunc
	counter int32
	closed  int32
	waitCh  chan struct{}
}

func (w *Wait) Cancel() {
	if atomic.CompareAndSwapInt32(&w.closed, 0, 1) {
		w.cancel()
	}
}

func (w *Wait) Wait() error {
	select {
	case <-w.ctx.Done():
		return w.ctx.Err()
	case <-w.waitCh:
		return nil
	}
	return nil
}

func (w *Wait) close() {
	if atomic.CompareAndSwapInt32(&w.closed, 0, 1) {
		close(w.waitCh)
	}
}

func (w *Wait) Done() {
	if atomic.LoadInt32(&w.closed) == 1 {
		return
	}
	if atomic.AddInt32(&w.counter, -1) < 1 {
		w.close()
	}
}

func newWait(parent context.Context, n int) *Wait {
	ctx, cancel := context.WithCancel(parent)
	return newWaitWithContextCancel(ctx, cancel, n)
}

func newWaitWithTimeout(parent context.Context, dur time.Duration, n int) *Wait {
	ctx, cancel := context.WithTimeout(parent, dur)
	return newWaitWithContextCancel(ctx, cancel, n)
}

func newWaitWithContextCancel(ctx context.Context, cancel context.CancelFunc, n int) *Wait {
	return &Wait{
		ctx:     ctx,
		cancel:  cancel,
		counter: int32(n),
		closed:  int32(0),
		waitCh:  make(chan struct{}),
	}
}

func WaitOne() *Wait {
	return WaitN(1)
}

func WaitTwo() *Wait {
	return WaitN(2)
}

func WaitN(n int) *Wait {
	return newWait(context.Background(), n)
}

func WaitTimeout(dur time.Duration, n int) *Wait {
	return newWaitWithTimeout(context.Background(), dur, n)
}

type WaitSequence struct {
	ctx    context.Context
	cancel context.CancelFunc
	wn     []*Wait
}

func (w *WaitSequence) Cancel() {
	w.cancel()
}

func (w *WaitSequence) Wait() error {
	for _, child := range w.wn {
		select {
		case <-w.ctx.Done():
			// main *Wait.Wait
			return w.ctx.Err()
		case <-child.ctx.Done():
			// child *Wait.Wait
			return child.ctx.Err()
		case <-child.waitCh:
			// nop. child success Done
		}
	}
	return nil
}

func newWaitSequence(parent context.Context, wn []*Wait) *WaitSequence {
	ctx, cancel := context.WithCancel(parent)
	return newWaitSequenceWithContextCancel(ctx, cancel, wn)
}

func newWaitSequenceWithTimeout(parent context.Context, dur time.Duration, wn []*Wait) *WaitSequence {
	ctx, cancel := context.WithTimeout(parent, dur)
	return newWaitSequenceWithContextCancel(ctx, cancel, wn)
}

func newWaitSequenceWithContextCancel(ctx context.Context, cancel context.CancelFunc, wn []*Wait) *WaitSequence {
	return &WaitSequence{
		ctx:    ctx,
		cancel: cancel,
		wn:     wn,
	}
}

func WaitSeq(wn ...*Wait) *WaitSequence {
	return newWaitSequence(context.Background(), wn)
}

func WaitSeqTimeout(dur time.Duration, wn ...*Wait) *WaitSequence {
	return newWaitSequenceWithTimeout(context.Background(), dur, wn)
}

type WaitRendezvous struct {
	ctx     context.Context
	cancel  context.CancelFunc
	counter int32
	closed  int32
	waitCh  chan struct{}
}

func (r *WaitRendezvous) Cancel() {
	r.cancel()
}

func (r *WaitRendezvous) Wait() error {
	if atomic.LoadInt32(&r.counter) < 1 {
		return nil // already reached
	}

	if atomic.AddInt32(&r.counter, -1) < 1 {
		close(r.waitCh)
	}

	select {
	case <-r.ctx.Done():
		return r.ctx.Err()
	case <-r.waitCh:
		return nil
	}
}

func newRendezvous(parent context.Context, n int) *WaitRendezvous {
	ctx, cancel := context.WithCancel(parent)
	return newRendezvousWithContextCancel(ctx, cancel, n)
}

func newRendezvousWithTimeout(parent context.Context, dur time.Duration, n int) *WaitRendezvous {
	ctx, cancel := context.WithTimeout(parent, dur)
	return newRendezvousWithContextCancel(ctx, cancel, n)
}

func newRendezvousWithContextCancel(ctx context.Context, cancel context.CancelFunc, n int) *WaitRendezvous {
	return &WaitRendezvous{
		ctx:     ctx,
		cancel:  cancel,
		counter: int32(n),
		closed:  int32(0),
		waitCh:  make(chan struct{}),
	}
}

func WaitRendez(n int) *WaitRendezvous {
	return newRendezvous(context.Background(), n)
}

func WaitRendezTimeout(dur time.Duration, n int) *WaitRendezvous {
	return newRendezvousWithTimeout(context.Background(), dur, n)
}

type WaitRequest struct {
	ctx      context.Context
	cancel   context.CancelFunc
	exchange chan interface{}
}

func (r *WaitRequest) Cancel() {
	r.cancel()
}

func (r *WaitRequest) Req(v interface{}) error {
	select {
	case <-r.ctx.Done():
		return r.ctx.Err()
	case r.exchange <- v:
		return nil
	}
}

func (r *WaitRequest) Wait() (interface{}, error) {
	select {
	case <-r.ctx.Done():
		return nil, r.ctx.Err()
	case v := <-r.exchange:
		return v, nil
	}
}

func newWaitRequest(parent context.Context) *WaitRequest {
	ctx, cancel := context.WithCancel(parent)
	return newWaitRequestWithContextCancel(ctx, cancel)
}

func newWaitRequestWithTimeout(parent context.Context, dur time.Duration) *WaitRequest {
	ctx, cancel := context.WithTimeout(parent, dur)
	return newWaitRequestWithContextCancel(ctx, cancel)
}

func newWaitRequestWithContextCancel(ctx context.Context, cancel context.CancelFunc) *WaitRequest {
	return &WaitRequest{
		ctx:      ctx,
		cancel:   cancel,
		exchange: make(chan interface{}),
	}
}

func WaitReq() *WaitRequest {
	return newWaitRequest(context.Background())
}

func WaitReqTimeout(dur time.Duration) *WaitRequest {
	return newWaitRequestWithTimeout(context.Background(), dur)
}

type WaitReplyFunc func(interface{}) (interface{}, error)

type WaitRequestReply struct {
	ctx      context.Context
	cancel   context.CancelFunc
	exchange chan *reqreplyRequest
}

type reqreplyRequest struct {
	value   interface{}
	replyCh chan *reqreplyReply
}

type reqreplyReply struct {
	value interface{}
	err   error
}

func (r *WaitRequestReply) Cancel() {
	r.cancel()
}

func (r *WaitRequestReply) Req(v interface{}) (interface{}, error) {
	replyCh := make(chan *reqreplyReply)

	req := &reqreplyRequest{
		value:   v,
		replyCh: replyCh,
	}

	// send req
	select {
	case <-r.ctx.Done():
		return nil, r.ctx.Err()

	case r.exchange <- req:
		// send ok
	}

	// recv reply
	select {
	case reply := <-replyCh:
		return reply.value, reply.err

	case <-r.ctx.Done():
		return nil, r.ctx.Err()
	}
}

func (r *WaitRequestReply) Reply(fn WaitReplyFunc) error {
	// recv req
	select {
	case <-r.ctx.Done():
		return r.ctx.Err()

	case req := <-r.exchange:
		// recv ok
		v, err := fn(req.value)

		reply := &reqreplyReply{
			value: v,
			err:   err,
		}

		// send reply
		select {
		case <-r.ctx.Done():
			return r.ctx.Err()
		case req.replyCh <- reply:
			// send ok
			close(req.replyCh)
			return nil
		}
	}
}

func newWaitRequestReply(parent context.Context) *WaitRequestReply {
	ctx, cancel := context.WithCancel(parent)
	return newWaitRequestReplyWithContextCancel(ctx, cancel)
}

func newWaitRequestReplyWithTimeout(parent context.Context, dur time.Duration) *WaitRequestReply {
	ctx, cancel := context.WithTimeout(parent, dur)
	return newWaitRequestReplyWithContextCancel(ctx, cancel)
}

func newWaitRequestReplyWithContextCancel(ctx context.Context, cancel context.CancelFunc) *WaitRequestReply {
	return &WaitRequestReply{
		ctx:      ctx,
		cancel:   cancel,
		exchange: make(chan *reqreplyRequest),
	}
}

func WaitReqReply() *WaitRequestReply {
	return newWaitRequestReply(context.Background())
}

func WaitReqReplyTimeout(dur time.Duration) *WaitRequestReply {
	return newWaitRequestReplyWithTimeout(context.Background(), dur)
}
