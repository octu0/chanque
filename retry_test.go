package chanque

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	eqDur := func(t *testing.T, expect, actual time.Duration) {
		if expect != actual {
			t.Errorf("not same expect:'%s' != actual:'%s'", expect, actual)
		}
	}
	rangeDur := func(t *testing.T, expectBase, expectMax, actual time.Duration) {
		if (expectBase <= actual && actual < expectMax) != true {
			t.Errorf("must jitter range '%s' <= n < '%s' (actual: '%s')", expectBase, expectMax, actual)
		}
	}
	t.Run("min=100ms/max=3s/NoJitter", func(tt *testing.T) {
		b := NewBackoffNoJitter(100*time.Millisecond, 3*time.Second)
		n1 := b.Next()
		n2 := b.Next()
		n3 := b.Next()
		n4 := b.Next()
		n5 := b.Next()
		n6 := b.Next()
		n7 := b.Next()
		n8 := b.Next()
		n9 := b.Next()
		n10 := b.Next()
		eqDur(tt, 100*time.Millisecond, n1)
		eqDur(tt, 200*time.Millisecond, n2)
		eqDur(tt, 400*time.Millisecond, n3)
		eqDur(tt, 800*time.Millisecond, n4)
		eqDur(tt, 1600*time.Millisecond, n5)
		eqDur(tt, 3000*time.Millisecond, n6) // truncate max
		eqDur(tt, 3000*time.Millisecond, n7)
		eqDur(tt, 3000*time.Millisecond, n8)
		eqDur(tt, 3000*time.Millisecond, n9)
		eqDur(tt, 3000*time.Millisecond, n10)
	})
	t.Run("min=100ms/max=3s/Jitter", func(tt *testing.T) {
		b := NewBackoff(100*time.Millisecond, 3*time.Second)
		n1 := b.Next()
		n2 := b.Next()
		n3 := b.Next()
		n4 := b.Next()
		n5 := b.Next()
		n6 := b.Next()
		n7 := b.Next()
		n8 := b.Next()
		n9 := b.Next()
		n10 := b.Next()
		eqDur(tt, 100*time.Millisecond, n1) // first = no jitter
		rangeDur(tt, 200*time.Millisecond, n3, n2)
		rangeDur(tt, 400*time.Millisecond, n4, n3)
		rangeDur(tt, 800*time.Millisecond, n5, n4)
		rangeDur(tt, 1600*time.Millisecond, n6, n5)
		eqDur(tt, 3000*time.Millisecond, n6) // truncate
		eqDur(tt, 3000*time.Millisecond, n7)
		eqDur(tt, 3000*time.Millisecond, n8)
		eqDur(tt, 3000*time.Millisecond, n9)
		eqDur(tt, 3000*time.Millisecond, n10)
	})
	t.Run("reset/min=100ms/max=3s/NoJitter", func(tt *testing.T) {
		b := NewBackoffNoJitter(100*time.Millisecond, 3*time.Second)
		n1 := b.Next()
		n2 := b.Next()
		n3 := b.Next()
		b.Reset()
		n4 := b.Next()
		n5 := b.Next()
		n6 := b.Next()
		n7 := b.Next()
		n8 := b.Next()
		n9 := b.Next()
		n10 := b.Next()
		b.Reset()
		n11 := b.Next()
		n12 := b.Next()
		n13 := b.Next()
		n14 := b.Next()
		n15 := b.Next()
		n16 := b.Next()
		eqDur(tt, 100*time.Millisecond, n1)
		eqDur(tt, 200*time.Millisecond, n2)
		eqDur(tt, 400*time.Millisecond, n3)
		eqDur(tt, 100*time.Millisecond, n4) // Reset
		eqDur(tt, 200*time.Millisecond, n5)
		eqDur(tt, 400*time.Millisecond, n6)
		eqDur(tt, 800*time.Millisecond, n7)
		eqDur(tt, 1600*time.Millisecond, n8)
		eqDur(tt, 3000*time.Millisecond, n9) // truncate max
		eqDur(tt, 3000*time.Millisecond, n10)
		eqDur(tt, 100*time.Millisecond, n11) // Reset
		eqDur(tt, 200*time.Millisecond, n12)
		eqDur(tt, 400*time.Millisecond, n13)
		eqDur(tt, 800*time.Millisecond, n14)
		eqDur(tt, 1600*time.Millisecond, n15)
		eqDur(tt, 3000*time.Millisecond, n16) // truncate
	})
}

func TestRetryFuture(t *testing.T) {
	t.Run("maxRetry", func(tt *testing.T) {
		neverSuccessFunc := func(context.Context) (interface{}, error) {
			return "test", fmt.Errorf("err on %s", tt.Name())
		}
		neverBreakErrorHandler := func(error, *Backoff) RetryNext {
			return RetryNextContinue
		}
		ctx := context.Background()
		e := NewExecutor(10, 10)
		defer e.Release()

		now := time.Now()
		b := NewBackoffNoJitter(10*time.Millisecond, 50*time.Millisecond)
		f := newRetryFuture(ctx, e)
		f.retry(b, 3, neverSuccessFunc, neverBreakErrorHandler)

		ve := f.Result()
		tt.Logf("retry running duration: %s", time.Since(now))

		err := ve.Err()
		if err == nil {
			tt.Fatalf("max retry happen!")
		}
		if strings.HasPrefix(err.Error(), "max retry exceed") != true {
			tt.Errorf("not max retry error: %s", err)
		}
		tt.Logf("last error = %s", err.Error())
		if ve.Value() != nil {
			tt.Errorf("max retry error will value nil")
		}
	})
	t.Run("context/timeout", func(tt *testing.T) {
		neverSuccessFunc := func(context.Context) (interface{}, error) {
			return "test", fmt.Errorf("err on %s", tt.Name())
		}
		neverBreakErrorHandler := func(error, *Backoff) RetryNext {
			return RetryNextContinue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		e := NewExecutor(10, 10)
		defer e.Release()

		now := time.Now()
		b := NewBackoffNoJitter(10*time.Millisecond, 500*time.Millisecond)
		f := newRetryFuture(ctx, e)
		f.retry(b, 10, neverSuccessFunc, neverBreakErrorHandler)

		time.Sleep(100 * time.Millisecond) // wait timeout

		ve := f.Result()
		tt.Logf("retry running duration: %s", time.Since(now))

		err := ve.Err()
		if err != ErrRetryContextTimeout {
			tt.Fatalf("context timeout happen!")
		}
		tt.Logf("last error = %s", err.Error())
		if ve.Value() != nil {
			tt.Errorf("context timeout error will value nil")
		}
	})
	t.Run("context/cancel", func(tt *testing.T) {
		neverSuccessFunc := func(context.Context) (interface{}, error) {
			return "test", fmt.Errorf("err on %s", tt.Name())
		}
		neverBreakErrorHandler := func(error, *Backoff) RetryNext {
			return RetryNextContinue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
		defer cancel()

		e := NewExecutor(10, 10)
		defer e.Release()

		now := time.Now()
		b := NewBackoffNoJitter(10*time.Millisecond, 500*time.Millisecond)
		f := newRetryFuture(ctx, e)
		f.retry(b, 10, neverSuccessFunc, neverBreakErrorHandler)

		time.Sleep(100 * time.Millisecond)
		cancel() // context cancel

		ve := f.Result()
		tt.Logf("retry running duration: %s", time.Since(now))

		err := ve.Err()
		if err != ErrRetryContextTimeout {
			tt.Fatalf("context cancel happen!")
		}
		tt.Logf("last error = %s", err.Error())
		if ve.Value() != nil {
			tt.Errorf("context cancel error will value nil")
		}
	})
	t.Run("delaySucc", func(tt *testing.T) {
		delayCounter := 0
		delaySuccessFunc := func(context.Context) (interface{}, error) {
			if delayCounter < 3 {
				delayCounter += 1
				return "test", fmt.Errorf("delaying %d", delayCounter)
			}
			return fmt.Sprintf("success %d", delayCounter), nil
		}
		neverBreakErrorHandler := func(error, *Backoff) RetryNext {
			return RetryNextContinue
		}
		ctx := context.Background()

		e := NewExecutor(10, 10)
		defer e.Release()

		now := time.Now()
		b := NewBackoffNoJitter(10*time.Millisecond, 500*time.Millisecond)
		f := newRetryFuture(ctx, e)
		f.retry(b, 10, delaySuccessFunc, neverBreakErrorHandler)

		ve := f.Result()
		tt.Logf("retry running duration: %s", time.Since(now))

		err := ve.Err()
		if err != nil {
			tt.Fatalf("covered maxRetry")
		}
		if ve.Value() != "success 3" {
			tt.Errorf("success and incre version not match: %v", ve.Value())
		}
	})
	t.Run("errHandler", func(tt *testing.T) {
		errorCount := 0
		counterErrorFunc := func(context.Context) (interface{}, error) {
			errorCount += 1
			return "test", fmt.Errorf("err %d", errorCount)
		}
		threeTimeErrorHandler := func(err error, b *Backoff) RetryNext {
			tt.Logf("%s", err.Error())
			if 3 < errorCount {
				return RetryNextBreak
			}
			return RetryNextContinue
		}
		ctx := context.Background()

		e := NewExecutor(10, 10)
		defer e.Release()

		now := time.Now()
		b := NewBackoffNoJitter(10*time.Millisecond, 500*time.Millisecond)
		f := newRetryFuture(ctx, e)
		f.retry(b, 10, counterErrorFunc, threeTimeErrorHandler)

		ve := f.Result()
		tt.Logf("retry running duration: %s", time.Since(now))

		err := ve.Err()
		if err == nil {
			tt.Fatalf("counter error happen!")
		}
		if err.Error() != "err 4" {
			tt.Errorf("maybe handler break not through: %s", err.Error())
		}
		tt.Logf("last error = %s", err.Error())
		if ve.Value() != nil {
			tt.Errorf("context cancel error will value nil")
		}
	})
}

func TestRetryNew(t *testing.T) {
	t.Run("default", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		r := NewRetry(e)
		if r.maxRetry != defaultRetryMaxRetry {
			tt.Errorf("init maxRetry: %d", r.maxRetry)
		}
		if r.backoff.jitter {
			tt.Errorf("init default jitter off")
		}
		if r.backoff.min != defaultRetryBackoffIntervalMin {
			tt.Errorf("init min default backoff: %s", r.backoff.min)
		}
		if r.backoff.max != defaultRetryBackoffIntervalMax {
			tt.Errorf("init max default backoff: %s", r.backoff.max)
		}
	})
	t.Run("option", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		myctx := context.Background()
		myctx = context.WithValue(myctx, "id", tt.Name())
		r := NewRetry(
			e,
			RetryContext(myctx),
			RetryMax(12),
			RetryBackoffIntervalMin(34*time.Millisecond),
			RetryBackoffIntervalMax(56*time.Millisecond),
			RetryBackoffUseJitter(true),
		)
		if r.ctx.Value("id") == nil {
			tt.Errorf("not my context: id nil")
		}
		if id, ok := r.ctx.Value("id").(string); ok != true {
			tt.Errorf("not my context: id: %v", r.ctx.Value("id"))
		} else {
			if id != tt.Name() {
				tt.Errorf("not my context: id: %s", id)
			}
		}
		if r.maxRetry != 12 {
			tt.Errorf("init option maxRetry: %d", r.maxRetry)
		}
		if r.backoff.jitter != true {
			tt.Errorf("init option jitter on")
		}
		if r.backoff.min != (34 * time.Millisecond) {
			tt.Errorf("init option min: %s", r.backoff.min)
		}
		if r.backoff.max != (56 * time.Millisecond) {
			tt.Errorf("init option max: %s", r.backoff.max)
		}
	})
	t.Run("backoff", func(tt *testing.T) {
		b := NewBackoff(123*time.Millisecond, 456*time.Millisecond)
		b.Next()
		b.Next()

		e := NewExecutor(10, 10)
		defer e.Release()

		r := NewRetryWithBackoff(
			e, b,
			RetryMax(7),
			RetryBackoffIntervalMin(8*time.Millisecond),
			RetryBackoffIntervalMax(9*time.Millisecond),
		)
		if r.backoff.min != (123 * time.Millisecond) {
			tt.Errorf("init my backoff min")
		}
		if r.backoff.max != (456 * time.Millisecond) {
			tt.Errorf("init my backoff max")
		}
		if r.backoff.jitter != true {
			tt.Errorf("init my backoff jitter")
		}
		if r.backoff.currentAttempt() != 2 {
			tt.Errorf("init my backoff reference")
		}
		if r.maxRetry != 7 {
			tt.Errorf("max retry use option")
		}
	})
}

func TestRetryRetry(t *testing.T) {
	t.Run("default/default", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		now := time.Now()
		counter := 0

		r := NewRetry(e)
		f := r.Retry(func(context.Context) (interface{}, error) {
			if counter < 3 {
				counter += 1
				return nil, fmt.Errorf("err: %d", counter)
			}
			return "hello world", nil
		})
		if (10 * time.Millisecond) < time.Since(now) {
			tt.Errorf("no blocking: %s", time.Since(now))
		}
		ve := f.Result()

		elapse := time.Since(now)
		tt.Logf("default retry running %s", elapse)

		if elapse < defaultRetryBackoffIntervalMin {
			tt.Errorf("elapsed time lesser than default min(retry 3): %s", elapse)
		}
		if defaultRetryBackoffIntervalMax < elapse {
			tt.Errorf("retryFunc not through?(retry 3): %s", elapse)
		}

		if ve.Value() == nil {
			tt.Errorf("retry exceed?")
		}
		if ve.Err() != nil {
			tt.Errorf("retry recovery: %v", ve.Err())
		}
		if v := ve.Value().(string); v != "hello world" {
			tt.Errorf("value != hello world: %v", ve.Value())
		}
	})

	t.Run("default/errorHandler", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		now := time.Now()
		counter := 0

		r := NewRetry(e)
		f := r.RetryWithErrorHandler(func(context.Context) (interface{}, error) {
			if counter < 3 {
				counter += 1
				return nil, fmt.Errorf("err[%d]", counter)
			}
			return "hello world", nil
		}, func(err error, b *Backoff) RetryNext {
			if 2 < counter {
				return RetryNextBreak
			}
			return RetryNextContinue
		})

		if (10 * time.Millisecond) < time.Since(now) {
			tt.Errorf("no blocking: %s", time.Since(now))
		}
		ve := f.Result()

		elapse := time.Since(now)
		tt.Logf("default retry running %s", elapse)

		if elapse < defaultRetryBackoffIntervalMin {
			tt.Errorf("elapsed time lesser than default min(retry 3): %s", elapse)
		}
		if defaultRetryBackoffIntervalMax < elapse {
			tt.Errorf("retryFunc not through?(retry 3): %s", elapse)
		}

		if ve.Value() != nil {
			tt.Errorf("retry cancel on handler")
		}
		if ve.Err() == nil {
			tt.Errorf("retry cancel handing")
		}
		if err := ve.Err(); err.Error() != "err[3]" {
			tt.Errorf("last err count: %v", err.Error())
		}
	})
}
