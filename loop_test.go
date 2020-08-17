package chanque

import (
	"testing"
	"time"
)

//func TestLoopExecute(t *testing.T) {
//	t.Run("noset", func(tt *testing.T) {
//		e := NewExecutor(1, 1)
//		defer e.Release()
//
//		lo := NewLoop(e)
//		lo.Execute()
//		lo.Stop()
//	})
//	t.Run("default", func(tt *testing.T) {
//		e := NewExecutor(1, 1)
//		defer e.Release()
//
//		lo := NewLoop(e)
//
//		c := 0
//		lo.SetDefault(func() LoopNext {
//			if c < 10 {
//				c += 1
//				time.Sleep(10 * time.Millisecond)
//				tt.Logf("next continue")
//				return LoopNextContinue
//			}
//			tt.Logf("next break")
//			return LoopNextBreak
//		})
//		lo.Execute()
//
//		time.Sleep(30 * time.Millisecond)
//
//		if (0 < c && c < 10) != true {
//			tt.Errorf("run default: %d", c)
//		}
//
//		time.Sleep(100 * time.Millisecond)
//
//		if c != 10 {
//			tt.Errorf("run default end: %d", c)
//		}
//
//		lo.StopAndWait()
//		time.Sleep(10 * time.Millisecond)
//
//		if e.Running() != 0 {
//			tt.Errorf("loop running: %d", e.Running())
//		}
//	})
//	t.Run("ticker", func(tt *testing.T) {
//		e := NewExecutor(1, 1)
//		defer e.Release()
//
//		lo := NewLoop(e)
//
//		c := 0
//		lo.SetTicker(func() LoopNext {
//			if c < 10 {
//				c += 1
//				tt.Logf("next continue")
//				return LoopNextContinue
//			}
//			tt.Logf("next break")
//			return LoopNextBreak
//		}, 10 * time.Millisecond)
//		lo.Execute()
//
//		time.Sleep(30 * time.Millisecond)
//
//		if (0 < c && c < 10) != true {
//			tt.Errorf("run ticker %d", c)
//		}
//
//		time.Sleep(100 * time.Millisecond)
//
//		if c != 10 {
//			tt.Errorf("run ticker end: %d", c)
//		}
//
//		lo.StopAndWait()
//		time.Sleep(10 * time.Millisecond)
//
//		if e.Running() != 0 {
//			tt.Errorf("loop running: %d", e.Running())
//		}
//	})
//	t.Run("queue", func(tt *testing.T) {
//		e := NewExecutor(1, 1)
//		defer e.Release()
//
//		lo := NewLoop(e)
//
//		q := NewQueue(0) // blocking queue
//		defer q.Close()
//
//		v := make([]string, 0)
//		lo.SetDequeue(func(val interface{}, ok bool) LoopNext {
//			if ok != true {
//				tt.Logf("queue closed")
//				return LoopNextBreak
//			}
//			tt.Logf("queue value = %v", val)
//			v = append(v, val.(string))
//			return LoopNextContinue
//		}, q)
//		lo.Execute()
//
//		go func() {
//			time.Sleep(30 * time.Millisecond)
//			q.Enqueue("hello")
//		}()
//		go func() {
//			time.Sleep(40 * time.Millisecond)
//			q.Enqueue("world")
//		}()
//
//		time.Sleep(100 * time.Millisecond)
//
//		if len(v) != 2 {
//			tt.Errorf("enqueue 2 times: %v", v)
//		}
//		if v[0] != "hello" || v[1] != "world" {
//			tt.Errorf("run dequeue: %v", v)
//		}
//
//		lo.StopAndWait()
//		time.Sleep(10 * time.Millisecond)
//
//		if e.Running() != 0 {
//			tt.Errorf("loop running: %d", e.Running())
//		}
//	})
//	t.Run("default/ticker", func(tt *testing.T) {
//		e := NewExecutor(1, 1)
//		defer e.Release()
//
//		lo := NewLoop(e)
//
//		c1, c2 := 0, 0
//		lo.SetDefault(func() LoopNext {
//			if c1 < 10 {
//				c1 += 1
//				time.Sleep(10 * time.Millisecond)
//				tt.Logf("next continue")
//				return LoopNextContinue
//			}
//			tt.Logf("next break")
//			return LoopNextBreak
//		})
//		lo.SetTicker(func() LoopNext {
//			if c2 < 10 {
//				c2 += 1
//				tt.Logf("next ticker continue")
//				return LoopNextContinue
//			}
//			tt.Logf("next ticker break")
//			return LoopNextBreak
//		}, 20 * time.Millisecond)
//		lo.Execute()
//
//		time.Sleep(50 * time.Millisecond)
//
//		if (0 < c1 && c1 < 10) != true {
//			tt.Errorf("run default: %d", c1)
//		}
//
//		if (0 <= c2 && c2 < 10) != true {
//			tt.Errorf("run ticker: %d", c2)
//		}
//
//		time.Sleep(100 * time.Millisecond)
//
//		if c1 != 10 {
//			tt.Errorf("run default end: %d", c1)
//		}
//		if c2 == 0 || c2 < 7 {
//			tt.Errorf("run ticker end: %d", c2)
//		}
//
//		lo.StopAndWait()
//		time.Sleep(10 * time.Millisecond)
//
//		if e.Running() != 0 {
//			tt.Errorf("loop running: %d", e.Running())
//		}
//	})
//
//	t.Run("default/queue", func(tt *testing.T) {
//		e := NewExecutor(1, 1)
//		defer e.Release()
//
//		lo := NewLoop(e)
//
//		q := NewQueue(0)
//		defer q.Close()
//
//		c := 0
//		v := make([]string, 0)
//		lo.SetDefault(func() LoopNext {
//			if c < 10 {
//				c += 1
//				time.Sleep(10 * time.Millisecond)
//				tt.Logf("next continue")
//				return LoopNextContinue
//			}
//			tt.Logf("next break")
//			return LoopNextBreak
//		})
//		lo.SetDequeue(func(val interface{}, ok bool) LoopNext {
//			if ok != true {
//				tt.Logf("queue closed")
//				return LoopNextBreak
//			}
//			tt.Logf("queue value = %v", val)
//			v = append(v, val.(string))
//			return LoopNextContinue
//		}, q)
//		lo.Execute()
//
//		go func() {
//			q.Enqueue("hello")
//		}()
//
//		go func() {
//			time.Sleep(30 * time.Millisecond)
//			q.Enqueue("world")
//		}()
//
//		time.Sleep(50 * time.Millisecond)
//
//		if (0 < c && c < 10) != true {
//			tt.Errorf("run default: %d", c)
//		}
//
//		time.Sleep(100 * time.Millisecond)
//
//		if c != 10 {
//			tt.Errorf("run default end: %d", c)
//		}
//
//		if len(v) != 2 {
//			tt.Errorf("run queue end: %d", len(v))
//		}
//		if v[0] != "hello" || v[1] != "world" {
//			tt.Errorf("run dequeue: %v", v)
//		}
//
//		lo.StopAndWait()
//		time.Sleep(10 * time.Millisecond)
//
//		if e.Running() != 0 {
//			tt.Errorf("loop running: %d", e.Running())
//		}
//	})
//	t.Run("ticker/queue", func(tt *testing.T) {
//		e := NewExecutor(1, 1)
//		defer e.Release()
//
//		lo := NewLoop(e)
//
//		q := NewQueue(0)
//		defer q.Close()
//
//		c := 0
//		v := make([]string, 0)
//		lo.SetTicker(func() LoopNext {
//			if c < 10 {
//				c += 1
//				tt.Logf("next continue")
//				return LoopNextContinue
//			}
//			tt.Logf("next break")
//			return LoopNextBreak
//		}, 5*time.Millisecond)
//		lo.SetDequeue(func(val interface{}, ok bool) LoopNext {
//			if ok != true {
//				tt.Logf("queue closed")
//				return LoopNextBreak
//			}
//			tt.Logf("queue value = %v", val)
//			v = append(v, val.(string))
//			return LoopNextContinue
//		}, q)
//		lo.Execute()
//
//		go func() {
//			q.Enqueue("hello")
//		}()
//
//		go func() {
//			time.Sleep(10 * time.Millisecond)
//			q.Enqueue("world")
//		}()
//
//		time.Sleep(10 * time.Millisecond)
//
//		if (0 < c && c < 10) != true {
//			tt.Errorf("run default: %d", c)
//		}
//
//		time.Sleep(100 * time.Millisecond)
//
//		if c != 10 {
//			tt.Errorf("run default end: %d", c)
//		}
//
//		if len(v) != 2 {
//			tt.Errorf("run queue end: %d", len(v))
//		}
//		if v[0] != "hello" || v[1] != "world" {
//			tt.Errorf("run dequeue: %v", v)
//		}
//
//		lo.StopAndWait()
//		time.Sleep(10 * time.Millisecond)
//
//		if e.Running() != 0 {
//			tt.Errorf("loop running: %d", e.Running())
//		}
//	})
//
//	t.Run("queue_with_close", func(tt *testing.T) {
//		e := NewExecutor(1, 1)
//		defer e.Release()
//
//		lo := NewLoop(e)
//
//		q := NewQueue(0, QueuePanicHandler(noopPanicHandler)) // blocking queue
//
//		v := make([]string, 0)
//		lo.SetDequeue(func(val interface{}, ok bool) LoopNext {
//			if ok != true {
//				tt.Logf("queue closed")
//				return LoopNextBreak
//			}
//			tt.Logf("queue value = %v", val)
//			v = append(v, val.(string))
//			return LoopNextContinue
//		}, q)
//		lo.Execute()
//
//		go func() {
//			time.Sleep(10 * time.Millisecond)
//			q.Enqueue("hello")
//		}()
//		go func() {
//			time.Sleep(50 * time.Millisecond)
//			q.Enqueue("world")
//		}()
//
//		time.Sleep(30 * time.Millisecond)
//
//		q.Close()
//
//		if len(v) != 1 {
//			tt.Errorf("enqueue 1 times: %v", v)
//		}
//		if v[0] != "hello" {
//			tt.Errorf("run dequeue: %v", v)
//		}
//
//		lo.StopAndWait()
//		time.Sleep(10 * time.Millisecond)
//
//		if e.Running() != 0 {
//			tt.Errorf("loop running: %d", e.Running())
//		}
//	})
//}
func TestLoopStop(t *testing.T) {
	t.Run("ticker", func(tt *testing.T) {
		e := NewExecutor(1, 1)
		defer e.Release()

		lo := NewLoop(e)
		c := 0
		lo.SetTicker(func() LoopNext {
			c += 1
			return LoopNextContinue
		}, 10*time.Millisecond)

		lo.Execute()

		time.Sleep(50 * time.Millisecond)

		lo.Stop()

		time.Sleep(10 * time.Millisecond)

		tt.Logf("ticker run %d times", c)
		if (0 < c && c < 6) != true {
			tt.Errorf("ticker running: %v", c)
		}

		if e.Running() != 0 {
			tt.Errorf("loop running: %d", e.Running())
		}
	})
	t.Run("queue", func(tt *testing.T) {
		e := NewExecutor(1, 1)
		defer e.Release()

		lo := NewLoop(e)
		q := NewQueue(0, QueuePanicHandler(noopPanicHandler))
		v := make([]string, 0)
		lo.SetDequeue(func(val interface{}, ok bool) LoopNext {
			v = append(v, val.(string))
			return LoopNextContinue
		}, q)

		lo.Execute()

		go func() {
			time.Sleep(10 * time.Millisecond)
			q.Enqueue("hello")
		}()

		go func() {
			time.Sleep(60 * time.Millisecond)
			q.Enqueue("world")
		}()

		time.Sleep(50 * time.Millisecond)

		lo.Stop()

		time.Sleep(10 * time.Millisecond)

		tt.Logf("queue run %v", v)
		if len(v) != 1 {
			tt.Errorf("stop happen")
		}
		if v[0] != "hello" {
			tt.Errorf("stop happen")
		}

		if e.Running() != 0 {
			tt.Errorf("loop running: %d", e.Running())
		}
	})
}
func TestLoopTimeout(t *testing.T) {
	t.Run("ticker", func(tt *testing.T) {
		e := NewExecutor(1, 1)
		defer e.Release()

		lo := NewLoop(e)
		c := 0
		lo.SetTicker(func() LoopNext {
			c += 1
			return LoopNextContinue
		}, 10*time.Millisecond)

		lo.ExecuteTimeout(50 * time.Millisecond)

		time.Sleep(100 * time.Millisecond)

		tt.Logf("ticker run %d times", c)
		if (0 < c && c < 6) != true {
			tt.Errorf("ticker running: %v", c)
		}

		if e.Running() != 0 {
			tt.Errorf("loop running: %d", e.Running())
		}
	})
	t.Run("queue", func(tt *testing.T) {
		e := NewExecutor(1, 1)
		defer e.Release()

		lo := NewLoop(e)
		q := NewQueue(0, QueuePanicHandler(noopPanicHandler))
		v := make([]string, 0)
		lo.SetDequeue(func(val interface{}, ok bool) LoopNext {
			v = append(v, val.(string))
			return LoopNextContinue
		}, q)

		lo.ExecuteTimeout(50 * time.Millisecond)

		go func() {
			time.Sleep(10 * time.Millisecond)
			q.Enqueue("hello")
		}()

		go func() {
			time.Sleep(60 * time.Millisecond)
			q.Enqueue("world")
		}()

		time.Sleep(100 * time.Millisecond)

		tt.Logf("queue run %v", v)
		if len(v) != 1 {
			tt.Errorf("stop happen")
		}
		if v[0] != "hello" {
			tt.Errorf("stop happen")
		}

		if e.Running() != 0 {
			tt.Errorf("loop running: %d", e.Running())
		}
	})
}
