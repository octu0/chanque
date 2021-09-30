package chanque

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestWaitOne(t *testing.T) {
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
		if err != context.Canceled {
			tt.Errorf("must cancel error")
		}
		tt.Logf("cancel = %s", err)
	})
	t.Run("cancel_done", func(tt *testing.T) {
		w := WaitOne()
		w.Cancel()
		w.Done()
		w.Done()

		err := w.Wait()
		if err != context.Canceled {
			tt.Errorf("must cancel error")
		}
		tt.Logf("cancel = %s", err)
	})
	t.Run("no_done", func(tt *testing.T) {
		w := WaitOne()
		defer w.Cancel()

		go func() {
			<-time.After(100 * time.Millisecond)
			tt.Logf("force cancel")
			w.Cancel()
		}()

		err := w.Wait()
		if err != context.Canceled {
			tt.Errorf("must cancel error")
		}
		tt.Logf("cancel = %s", err)
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

func TestWaitTwo(t *testing.T) {
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

		go func() {
			<-time.After(100 * time.Millisecond)
			tt.Logf("force cancel")
			w.Cancel()
		}()

		err := w.Wait()
		if err != context.Canceled {
			tt.Errorf("must cancel error")
		}
		tt.Logf("cancel = %s", err)
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
		if err != context.Canceled {
			tt.Errorf("must cancel error")
		}
		tt.Logf("cancel = %s", err)
	})
	t.Run("cancel_done", func(tt *testing.T) {
		w := WaitTwo()
		w.Cancel()
		w.Done()
		w.Done()
		w.Done()

		err := w.Wait()
		if err != context.Canceled {
			tt.Errorf("must cancel error")
		}
		tt.Logf("cancel = %s", err)
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

		go func() {
			<-time.After(100 * time.Millisecond)
			tt.Logf("force cancel")
			w.Cancel()
		}()

		err := w.Wait()
		if err != context.Canceled {
			tt.Errorf("must cancel error")
		}
		tt.Logf("cancel = %s", err)
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
		if err != context.Canceled {
			tt.Errorf("must cancel error")
		}
		tt.Logf("cancel = %s", err)
	})
}

func TestWaitTimeout(t *testing.T) {
	t.Run("no_done", func(tt *testing.T) {
		w := WaitTimeout(10*time.Millisecond, 1)
		defer w.Cancel()

		err := w.Wait()
		if err != context.DeadlineExceeded {
			tt.Errorf("must timeout error")
		}
		tt.Logf("timeout = %s", err)
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
		if err != context.DeadlineExceeded {
			tt.Errorf("must timeout error")
		}
		tt.Logf("timeout = %s", err)
	})
}

func TestWaitSeq(t *testing.T) {
	t.Run("wait_one", func(tt *testing.T) {
		w1 := WaitN(1)
		defer w1.Cancel()

		ws := WaitSeq(w1)
		defer ws.Cancel()

		v := int32(0)
		f := func(_w *Wait, i *int32) {
			defer _w.Done()
			atomic.AddInt32(i, 123)
		}
		go f(w1, &v)

		if err := ws.Wait(); err != nil {
			tt.Errorf("must no error")
		}
		if v != 123 {
			tt.Errorf("goroutine update = %v", v)
		}
	})
	t.Run("done_child_one", func(tt *testing.T) {
		w1 := WaitN(1)
		defer w1.Cancel()

		w2 := WaitN(1)
		defer w2.Cancel()

		w3 := WaitN(1)
		defer w3.Cancel()

		w2.Done()

		ws := WaitSeq(w1, w2, w3)
		defer ws.Cancel()

		go func() {
			<-time.After(100 * time.Millisecond)
			tt.Logf("force cancel")
			ws.Cancel()
		}()

		err := ws.Wait()
		if err != context.Canceled {
			tt.Errorf("must cancel error")
		}
		tt.Logf("cancel = %s", err)
	})
	t.Run("done_child_all", func(tt *testing.T) {
		w1 := WaitN(1)
		defer w1.Cancel()

		w2 := WaitN(1)
		defer w2.Cancel()

		w3 := WaitN(1)
		defer w3.Cancel()

		w1.Done()
		w2.Done()
		w3.Done()

		ws := WaitSeq(w1, w2, w3)
		defer ws.Cancel()

		if err := ws.Wait(); err != nil {
			tt.Errorf("must no error")
		}
	})
}

func TestWaitSeqTimeout(t *testing.T) {
	t.Run("no_done", func(tt *testing.T) {
		w1 := WaitN(1)
		defer w1.Done()
		w2 := WaitN(1)
		defer w2.Done()

		ws := WaitSeqTimeout(10*time.Millisecond, w1, w2)

		w2.Done()

		err := ws.Wait()
		if err != context.DeadlineExceeded {
			tt.Errorf("must timeout error")
		}
		tt.Logf("timeout = %s", err)
	})
	t.Run("child_timeout", func(tt *testing.T) {
		w1 := WaitTimeout(10*time.Millisecond, 1)
		defer w1.Done()
		w2 := WaitN(1)
		defer w2.Done()

		ws := WaitSeqTimeout(100*time.Millisecond, w1, w2)
		now := time.Now()
		err := ws.Wait()
		elapse := time.Since(now)

		if err != context.DeadlineExceeded {
			tt.Errorf("must timeout error")
		}
		tt.Logf("timeout = %s", err)

		if (100 * time.Millisecond) <= elapse {
			tt.Errorf("must catch w1 timeout")
		}
		tt.Logf("elapse = %s", elapse)
	})
}

func TestWaitRendez(t *testing.T) {
	t.Run("wait", func(tt *testing.T) {
		wz := WaitRendez(2)
		defer wz.cancel()

		w := WaitN(2)

		v1 := 0
		v2 := 0
		go func() { // goroutine1
			defer w.Done()
			if err := wz.Wait(); err != nil {
				tt.Errorf(err.Error())
			}

			v1 = 100
		}()

		// wait run goroutine1
		time.Sleep(100 * time.Millisecond)

		go func() { // goroutine2
			defer w.Done()
			if err := wz.Wait(); err != nil {
				tt.Errorf(err.Error())
			}

			v2 = 200
		}()

		if v1 != 0 {
			tt.Errorf("wait rendezvous")
		}
		if v2 != 0 {
			tt.Errorf("wait rendezvous")
		}

		if err := w.Wait(); err != nil {
			tt.Errorf("must no error")
		}
		if v1 != 100 {
			tt.Errorf("goroutine1")
		}
		if v2 != 200 {
			tt.Errorf("goroutine2")
		}
	})
	t.Run("cancel", func(tt *testing.T) {
		wz := WaitRendez(2)
		defer wz.cancel()

		w := WaitN(1)

		go func() {
			defer w.Done()
			err := wz.Wait()
			if err != context.Canceled {
				tt.Errorf("must cancel error")
			}
			tt.Logf("cancel = %s", err)
		}()

		wz.Cancel()
		w.Wait()
	})
}

func TestWaitRendezTimeout(t *testing.T) {
	t.Run("enough_wait_child", func(tt *testing.T) {
		wz := WaitRendezTimeout(10*time.Millisecond, 2)
		defer wz.Cancel()
		w := WaitN(1)
		defer w.Cancel()

		go func() {
			defer w.Done()

			err := wz.Wait()
			if err != context.DeadlineExceeded {
				tt.Errorf("must timeout error")
			}
			tt.Logf("timeout = %s", err)
		}()

		w.Wait()
	})
}

func TestWaitRequest(t *testing.T) {
	t.Run("wait", func(tt *testing.T) {
		wr := WaitReq()
		defer wr.Cancel()

		go func() {
			if err := wr.Req("hello world"); err != nil {
				tt.Errorf("must no error")
			}
		}()

		v, err := wr.Wait()
		if err != nil {
			tt.Errorf("must no error")
		}
		if "hello world" != v.(string) {
			tt.Errorf("no req value")
		}
	})
	t.Run("cancel", func(tt *testing.T) {
		wr := WaitReq()
		w := WaitN(1)

		go func() {
			defer w.Done()

			err := wr.Req("hello world")
			if err != context.Canceled {
				tt.Errorf("must cancel error")
			}
			tt.Logf("cancel = %s", err)
		}()

		wr.Cancel()

		v, err := wr.Wait()
		if err != context.Canceled {
			tt.Errorf("must cancel error")
		}
		if v != nil {
			tt.Errorf("must no value")
		}
	})
	t.Run("no_req", func(tt *testing.T) {
		wr := WaitReq()

		go func() {
			<-time.After(100 * time.Millisecond)
			tt.Logf("force cancel")
			wr.Cancel()
		}()

		v, err := wr.Wait()
		if err != context.Canceled {
			tt.Logf("cancel = %s", err)
		}
		if v != nil {
			tt.Errorf("must no value")
		}
	})
}

func TestWaitRequestTimeout(t *testing.T) {
	t.Run("no_wait", func(tt *testing.T) {
		wr := WaitReqTimeout(10 * time.Millisecond)
		defer wr.Cancel()
		w := WaitN(1)
		defer w.Cancel()

		go func() {
			defer w.Done()

			err := wr.Req("test")
			if err != context.DeadlineExceeded {
				tt.Errorf("must timeout error")
			}
			tt.Logf("timeout = %s", err)
		}()

		w.Wait()
	})
	t.Run("no_req", func(tt *testing.T) {
		wr := WaitReqTimeout(10 * time.Millisecond)
		defer wr.Cancel()
		w := WaitN(1)
		defer w.Cancel()

		go func() {
			defer w.Done()

			_, err := wr.Wait()
			if err != context.DeadlineExceeded {
				tt.Errorf("must timeout error")
			}
			tt.Logf("timeout = %s", err)
		}()

		w.Wait()
	})
}

func TestWaitReqReply(t *testing.T) {
	t.Run("reqreply", func(tt *testing.T) {
		wrr := WaitReqReply()
		w := WaitN(2)

		go func() {
			defer w.Done()

			v, err := wrr.Req("hello")
			if err != nil {
				tt.Errorf("must no error")
			}

			if "helloworld" != v.(string) {
				tt.Errorf("reply value")
			}
			tt.Logf("reply = %v", v)
		}()

		go func() {
			defer w.Done()

			err := wrr.Reply(func(v interface{}) (interface{}, error) {
				if "hello" != v.(string) {
					tt.Errorf("req value")
				}
				tt.Logf("req = %v", v)
				return v.(string) + "world", nil
			})
			if err != nil {
				tt.Errorf("must no error")
			}
		}()

		if err := w.Wait(); err != nil {
			tt.Errorf("must no error")
		}
	})
	t.Run("cancel", func(tt *testing.T) {
		wrr := WaitReqReply()
		w := WaitN(2)

		go func() {
			defer w.Done()

			_, err := wrr.Req(nil)
			if err != context.Canceled {
				tt.Errorf("must cancel error")
			}
			tt.Logf("cancel = %s", err)
		}()

		go func() {
			defer w.Done()

			err := wrr.Reply(nil)
			if err != context.Canceled {
				tt.Errorf("must cancel error")
			}
			tt.Logf("cancel = %s", err)
		}()

		wrr.Cancel()
		w.Wait()
	})
	t.Run("reply_err", func(tt *testing.T) {
		wrr := WaitReqReply()
		w := WaitN(2)

		errMyCustomError := errors.New("test")
		go func() {
			defer w.Done()

			_, err := wrr.Req(nil)
			if err != errMyCustomError {
				tt.Errorf("must errMyCustomError")
			}
		}()

		go func() {
			defer w.Done()

			err := wrr.Reply(func(v interface{}) (interface{}, error) {
				return nil, errMyCustomError
			})
			if err != nil {
				tt.Errorf("must no error")
			}
		}()

		w.Wait()
	})
	t.Run("cancel_reply", func(tt *testing.T) {
		wrr := WaitReqReply()
		w := WaitN(2)

		go func() {
			defer w.Done()

			_, err := wrr.Req("test")
			if err != context.Canceled {
				tt.Errorf("must cancel error")
			}
			tt.Logf("cancel = %s", err)
		}()

		go func() {
			defer w.Done()

			err := wrr.Reply(func(v interface{}) (interface{}, error) {
				if v == nil {
					tt.Errorf("must req value")
				}
				if v.(string) != "test" {
					tt.Errorf("recv req value")
				}
				wrr.Cancel() // cancel
				return nil, nil
			})
			if err != context.Canceled {
				tt.Errorf("must cancel error")
			}
			tt.Logf("cancel = %s", err)
		}()

		wrr.Cancel()
		w.Wait()
	})
}

func TestWaitReqReplyTimeout(t *testing.T) {
	t.Run("no_reply", func(tt *testing.T) {
		wrr := WaitReqReplyTimeout(10 * time.Millisecond)
		defer wrr.Cancel()

		w := WaitN(1)
		defer w.Cancel()

		go func() {
			defer w.Done()

			_, err := wrr.Req("test")
			if err != context.DeadlineExceeded {
				tt.Errorf("must timeout error")
			}
			tt.Logf("timeout = %s", err)
		}()

		w.Wait()
	})
	t.Run("no_req", func(tt *testing.T) {
		wrr := WaitReqReplyTimeout(10 * time.Millisecond)
		defer wrr.Cancel()

		w := WaitN(1)
		defer w.Cancel()

		go func() {
			defer w.Done()

			err := wrr.Reply(func(v interface{}) (interface{}, error) {
				return "test", nil
			})
			if err != context.DeadlineExceeded {
				tt.Errorf("must timeout error")
			}
			tt.Logf("timeout = %s", err)
		}()

		w.Wait()
	})
}
