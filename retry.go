package chanque

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	cryptoRnd "crypto/rand"
)

const (
	maxInt64 = int(math.MaxInt64)
)

var (
	RetryRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func init() {
	buf := make([]byte, 8)
	_, err := io.ReadFull(cryptoRnd.Reader, buf[:])
	if err != nil {
		return
	}
	n := int64(binary.LittleEndian.Uint64(buf[:]))
	RetryRand = rand.New(rand.NewSource(n))
}

type Backoff struct {
	min     time.Duration
	max     time.Duration
	jitter  bool
	attempt uint64
}

func NewBackoff(min, max time.Duration) *Backoff {
	return newBackoff(min, max, true)
}

func NewBackoffNoJitter(min, max time.Duration) *Backoff {
	return newBackoff(min, max, false)
}

func newBackoff(min, max time.Duration, useJitter bool) *Backoff {
	if min < 1 {
		min = 1
	}
	if max < 1 {
		max = 1
	}
	if max < min {
		max = min
	}

	return &Backoff{
		min:     min,
		max:     max,
		jitter:  useJitter,
		attempt: 0,
	}
}

func (b *Backoff) Next() time.Duration {
	n := atomic.AddUint64(&b.attempt, 1) - 1
	pow := 1 << n // math.Pow(2, n). binary exponential
	if maxInt64 <= pow {
		pow = maxInt64 - 1
	}

	powDur := time.Duration(pow)
	dur := b.min * powDur
	if dur <= 0 {
		dur = math.MaxInt64 // overflow
	}
	if b.jitter {
		size := int64(dur - b.min)
		if 0 < size {
			// must 1 < attempt
			rnd := RetryRand.Int63n(size)
			dur += time.Duration(rnd)
			if dur <= 0 {
				dur = math.MaxInt64 // overflow
			}
		}
	}

	if b.max <= dur {
		dur = b.max // truncate
	}
	return dur
}

func (b *Backoff) Reset() {
	atomic.StoreUint64(&b.attempt, 0)
}

func (b *Backoff) currentAttempt() uint64 {
	return atomic.LoadUint64(&b.attempt)
}

const (
	defaultRetryBackoffIntervalMin time.Duration = 300 * time.Millisecond
	defaultRetryBackoffIntervalMax time.Duration = 60 * time.Second
	defaultRetryMaxRetry           int           = 10
)

type RetryNext uint8

const (
	RetryNextContinue RetryNext = iota + 1
	RetryNextBreak
)

type (
	RetryFunc         func(context.Context) (interface{}, error)
	RetryErrorHandler func(err error, b *Backoff) RetryNext
)

func defaultErrorHandler(err error, b *Backoff) RetryNext {
	if err != nil {
		return RetryNextContinue
	}
	return RetryNextBreak
}

type RetryOptionFunc func(*optRetry)

type optRetry struct {
	ctx                context.Context
	maxRetry           int
	backoffIntervalMin time.Duration
	backoffIntervalMax time.Duration
	backoffUseJitter   bool
}

func RetryContext(ctx context.Context) RetryOptionFunc {
	return func(opt *optRetry) {
		opt.ctx = ctx
	}
}

func RetryMax(max int) RetryOptionFunc {
	return func(opt *optRetry) {
		opt.maxRetry = max
	}
}

func RetryBackoffIntervalMin(dur time.Duration) RetryOptionFunc {
	return func(opt *optRetry) {
		opt.backoffIntervalMin = dur
	}
}

func RetryBackoffIntervalMax(dur time.Duration) RetryOptionFunc {
	return func(opt *optRetry) {
		opt.backoffIntervalMax = dur
	}
}

func RetryBackoffUseJitter(useJitter bool) RetryOptionFunc {
	return func(opt *optRetry) {
		opt.backoffUseJitter = useJitter
	}
}

var (
	ErrRetryNotComplete    = errors.New("RetryFuture retry not complete")
	ErrRetryContextTimeout = errors.New("Retry context timeout or canceled")
)

type Retry struct {
	executor *Executor
	backoff  *Backoff
	ctx      context.Context
	maxRetry int
}

func NewRetry(executor *Executor, funcs ...RetryOptionFunc) *Retry {
	return newRetry(executor, nil, funcs...)
}

func NewRetryWithBackoff(executor *Executor, backoff *Backoff, funcs ...RetryOptionFunc) *Retry {
	return newRetry(executor, backoff, funcs...)
}

func newRetry(executor *Executor, backoff *Backoff, funcs ...RetryOptionFunc) *Retry {
	opt := new(optRetry)
	for _, fn := range funcs {
		fn(opt)
	}

	if opt.maxRetry < 1 {
		opt.maxRetry = defaultRetryMaxRetry
	}
	if opt.backoffIntervalMin < 1 {
		opt.backoffIntervalMin = defaultRetryBackoffIntervalMin
	}
	if opt.backoffIntervalMax < 1 {
		opt.backoffIntervalMax = defaultRetryBackoffIntervalMax
	}

	if opt.ctx == nil {
		opt.ctx = context.Background()
	}
	if backoff == nil {
		backoff = newBackoff(opt.backoffIntervalMin, opt.backoffIntervalMax, opt.backoffUseJitter)
	}

	return &Retry{
		executor: executor,
		backoff:  backoff,
		ctx:      opt.ctx,
		maxRetry: opt.maxRetry,
	}
}

func (r *Retry) Retry(fn RetryFunc) *RetryFuture {
	return r.RetryWithErrorHandler(fn, defaultErrorHandler)
}

func (r *Retry) RetryWithErrorHandler(fn RetryFunc, eh RetryErrorHandler) *RetryFuture {
	f := newRetryFuture(r.ctx, r.executor)
	f.retry(r.backoff, r.maxRetry, fn, eh)
	return f
}

type RetryFuture struct {
	once    *sync.Once
	ctx     context.Context
	subExec *SubExecutor
	result  ValueError
}

func newRetryFuture(ctx context.Context, executor *Executor) *RetryFuture {
	return &RetryFuture{
		once:    new(sync.Once),
		ctx:     ctx,
		subExec: executor.SubExecutor(),
		result:  &tupleValueError{value: nil, err: ErrRetryNotComplete},
	}
}

func (f *RetryFuture) retry(b *Backoff, maxRetry int, fn RetryFunc, eh RetryErrorHandler) {
	f.subExec.Submit(func() {
		var lastValue interface{}
		var lastError error
		defer func() {
			f.result = &tupleValueError{value: lastValue, err: lastError}
		}()

		for i := 0; i < maxRetry; i += 1 {
			select {
			case <-f.ctx.Done():
				// on context canceled or timeout
				lastValue = nil
				lastError = ErrRetryContextTimeout
				return
			default:
				// context live
			}

			value, err := fn(f.ctx)
			if err == nil {
				lastValue = value
				lastError = nil
				return
			}

			lastError = err
			next := eh(err, b)
			switch next {
			case RetryNextContinue:
				time.Sleep(b.Next())
			case RetryNextBreak:
				return
			}
		}
		lastValue = nil
		lastError = fmt.Errorf("max retry exceed: %w", lastError)
	})
}

func (f *RetryFuture) Result() ValueError {
	f.once.Do(func() {
		f.subExec.Wait()
	})
	return f.result
}
