package chanque

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestWaitWaitOne(t *testing.T) {
	t.Run("wait", func(tt *testing.T) {
		w := WaitOne()
		defer w.Cancel()

		v := int32(0)
		f := func(_w *Wait, i *int32) {
			defer _w.Done()
			atomic.AddInt32(i, 123)
		}
		go f(w, &v)

		if err := w.Wait(); err != nil {
			tt.Errorf("must no error")
		}
		if v != 123 {
			tt.Errorf("goroutine update = %v", v)
		}
	})
	t.Run("done", func(tt *testing.T) {
		w := WaitOne()
		defer w.Cancel()

		w.Done()
		if err := w.Wait(); err != nil {
			tt.Errorf("must no error")
		}
	})
	t.Run("cancel", func(tt *testing.T) {
		w := WaitOne()
		w.Cancel()
		err := w.Wait()
		if err == nil {
			tt.Errorf("must cancel error")
		}
		tt.Logf("cancel = %s", err.Error())
	})
	t.Run("cancel_done", func(tt *testing.T) {
		w := WaitOne()
		w.Cancel()
		w.Done()
		w.Done()

		err := w.Wait()
		if err == nil {
			tt.Errorf("must cancel error")
		}
		tt.Logf("cancel = %s", err.Error())
	})
	t.Run("no_done", func(tt *testing.T) {
		w := WaitOne()
		defer w.Cancel()

		if err := w.WaitTimeout(100 * time.Millisecond); err != ErrWaitingTimeout {
			tt.Errorf("must wait timeout")
		}
	})
	t.Run("done_twice", func(tt *testing.T) {
		w := WaitOne()
		defer w.Cancel()

		w.Done()
		w.Done()
		if err := w.Wait(); err != nil {
			tt.Errorf("must no error")
		}
	})
}

func TestWaitWaitTwo(t *testing.T) {
	t.Run("wait", func(tt *testing.T) {
		w := WaitTwo()
		defer w.Cancel()

		v := int32(0)
		f := func(_w *Wait, i *int32) {
			defer _w.Done()
			atomic.AddInt32(i, 1)
		}
		go f(w, &v)
		go f(w, &v)

		if err := w.Wait(); err != nil {
			tt.Errorf("must no error")
		}
		if v != 2 {
			tt.Errorf("goroutine update = %v", v)
		}
	})
	t.Run("done_once", func(tt *testing.T) {
		w := WaitTwo()
		defer w.Cancel()

		w.Done()
		if err := w.WaitTimeout(100 * time.Millisecond); err != ErrWaitingTimeout {
			tt.Errorf("must wait timeout")
		}
	})
	t.Run("done", func(tt *testing.T) {
		w := WaitTwo()
		defer w.Cancel()

		w.Done()
		w.Done()
		if err := w.Wait(); err != nil {
			tt.Errorf("must no error")
		}
	})
	t.Run("cancel", func(tt *testing.T) {
		w := WaitTwo()
		w.Cancel()
		err := w.Wait()
		if err == nil {
			tt.Errorf("must cancel error")
		}
		tt.Logf("cancel = %s", err.Error())
	})
	t.Run("cancel_done", func(tt *testing.T) {
		w := WaitTwo()
		w.Cancel()
		w.Done()
		w.Done()
		w.Done()

		err := w.Wait()
		if err == nil {
			tt.Errorf("must cancel error")
		}
		tt.Logf("cancel = %s", err.Error())
	})
}

func TestWaitN(t *testing.T) {
	t.Run("wait", func(tt *testing.T) {
		n := 1000
		w := WaitN(n)
		defer w.Cancel()

		v := int32(0)
		f := func(_w *Wait, i *int32) {
			defer _w.Done()
			atomic.AddInt32(i, 1)
		}
		for i := 0; i < n; i += 1 {
			go f(w, &v)
		}
		if err := w.Wait(); err != nil {
			tt.Errorf("must no error")
		}
		if v != int32(n) {
			tt.Errorf("goroutine update = %v", v)
		}
	})
	t.Run("done_once", func(tt *testing.T) {
		n := 1000
		w := WaitN(n)
		defer w.Cancel()

		w.Done()
		if err := w.WaitTimeout(100 * time.Millisecond); err != ErrWaitingTimeout {
			tt.Errorf("must wait timeout")
		}
	})
	t.Run("done", func(tt *testing.T) {
		n := 1000
		w := WaitN(n)
		defer w.Cancel()

		for i := 0; i < n; i += 1 {
			w.Done()
		}
		if err := w.Wait(); err != nil {
			tt.Errorf("must no error")
		}
	})
	t.Run("cancel", func(tt *testing.T) {
		n := 1000
		w := WaitN(n)
		w.Cancel()
		err := w.Wait()
		if err == nil {
			tt.Errorf("must cancel error")
		}
		tt.Logf("cancel = %s", err.Error())
	})
	t.Run("cancel_done", func(tt *testing.T) {
		n := 1000
		w := WaitN(n)
		w.Cancel()
		for i := 0; i < n; i += 1 {
			w.Done()
		}
		w.Done() // N + 1

		err := w.Wait()
		if err == nil {
			tt.Errorf("must cancel error")
		}
		tt.Logf("cancel = %s", err.Error())
	})
}

func TestWaitTimeout(t *testing.T) {
	t.Run("wait", func(tt *testing.T) {
		w := WaitTimeout(1*time.Second, 1)
		defer w.Cancel()

		v := int32(0)
		f := func(_w *Wait, i *int32) {
			defer _w.Done()
			atomic.AddInt32(i, 123)
		}
		go f(w, &v)

		if err := w.Wait(); err != nil {
			tt.Errorf("must no error")
		}
		if v != 123 {
			tt.Errorf("goroutine update = %v", v)
		}
	})
	t.Run("done", func(tt *testing.T) {
		w := WaitTimeout(1*time.Second, 1)
		defer w.Cancel()

		w.Done()
		if err := w.Wait(); err != nil {
			tt.Errorf("must no error")
		}
	})
	t.Run("cancel", func(tt *testing.T) {
		w := WaitTimeout(1*time.Second, 1)
		w.Cancel()
		err := w.Wait()
		if err == nil {
			tt.Errorf("must cancel error")
		}
		tt.Logf("cancel = %s", err.Error())
	})
	t.Run("cancel_done", func(tt *testing.T) {
		w := WaitTimeout(1*time.Second, 1)
		w.Cancel()
		w.Done()
		w.Done()

		err := w.Wait()
		if err == nil {
			tt.Errorf("must cancel error")
		}
		tt.Logf("cancel = %s", err.Error())
	})
	t.Run("no_done", func(tt *testing.T) {
		w := WaitTimeout(10*time.Millisecond, 1)
		defer w.Cancel()

		err := w.Wait()
		if err == nil {
			tt.Errorf("must timeout error")
		}
		tt.Logf("timeout = %s", err.Error())
	})
	t.Run("done_twice", func(tt *testing.T) {
		w := WaitTimeout(10*time.Millisecond, 1)
		defer w.Cancel()

		w.Done()
		w.Done()
		if err := w.Wait(); err != nil {
			tt.Errorf("must no error")
		}
	})
	t.Run("wait_timeout", func(tt *testing.T) {
		w := WaitTimeout(100*time.Millisecond, 1)
		defer w.Cancel()

		f := func(_w *Wait) {
			defer _w.Done()
			time.Sleep(500 * time.Millisecond)
		}
		go f(w)

		err := w.Wait()
		if err == nil {
			tt.Errorf("must timeout error")
		}
		tt.Logf("timeout = %s", err.Error())
	})
}
