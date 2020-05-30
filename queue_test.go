package chanque

import(
  "testing"
  "time"
  "sync"
)

func TestQueueLenCap(t *testing.T) {
  t.Run("cap0", func(tt *testing.T) {
    q := NewQueue(0)
    if q.Len() != 0 {
      tt.Errorf("no enqueue")
    }
    if q.Cap() != 0 {
      tt.Errorf("initial cap is 0")
    }
  })
  t.Run("cap10", func(tt *testing.T) {
    q := NewQueue(10)
    if q.Len() != 0 {
      tt.Errorf("no enqueue")
    }
    if q.Cap() != 10 {
      tt.Errorf("initial cap is 10")
    }
  })
  t.Run("cap0len", func(tt *testing.T) {
    q := NewQueue(0)
    latch := make(chan struct{})
    go func(){
      <-latch
      // requires reader
      v, _ := q.Dequeue()
      tt.Logf(v.(string))
    }()

    latch <-struct{}{}
    if q.Len() != 0 {
      tt.Errorf("no enqueue")
    }

    latch2 := make(chan struct{})
    go func() {
      <-latch2
      q.Enqueue("hello world")
    }()

    latch2 <-struct{}{}
    if q.Len() != 0 {
      tt.Errorf("cap0 len always 0")
    }
  })
  t.Run("cap10len", func(tt *testing.T) {
    q := NewQueue(10)
    if q.Len() != 0 {
      tt.Errorf("no enqueue")
    }
    if q.Cap() != 10 {
      tt.Errorf("capacity 10")
    }

    for i := 0; i < 5; i += 1{
      q.Enqueue("1")
    }

    if q.Len() != 5 {
      tt.Errorf("enqueue 5 times")
    }
    if q.Cap() != 10 {
      tt.Errorf("capacity 10")
    }
    wg := new(sync.WaitGroup)
    for i := 0; i < 3; i += 1 {
      wg.Add(1)
      go func(w *sync.WaitGroup){
        q.Dequeue()
        w.Done()
      }(wg)
    }
    wg.Wait()
    if q.Len() != 2 {
      tt.Errorf("dequeue 3 times")
    }
    if q.Cap() != 10 {
      tt.Errorf("capacity 10")
    }
  })
}

func TestBlockingEnqueue(t *testing.T) {
  t.Run("blocking", func(tt *testing.T) {
    done  := make(chan struct{})
    queue := NewQueue(0, QueuePanicHandler(noopPanicHandler))

    go func(){
      queue.Enqueue(struct{}{})
      done <-struct{}{}
    }()

    select {
    case <-time.After(10 * time.Millisecond):
      tt.Log("blocking enqueue ok1")
      queue.Close()

    case <-done:
      tt.Errorf("not blocking enqueue 1")
    }
  })

  t.Run("non-blocking", func(tt *testing.T) {
    done  := make(chan struct{})
    queue := NewQueue(1, QueuePanicHandler(noopPanicHandler))

    go func(){
      queue.Enqueue(struct{}{})
      done <-struct{}{}
    }()

    select {
    case <-time.After(10 * time.Millisecond):
      queue.Close()
      tt.Errorf("blocking enqueue: queue2 has free capacity")

    case <-done:
      tt.Log("blocking enqueue ok2")
    }
  })
}

func TestBlockingEnqueueNB(t *testing.T) {
  t.Run("size0", func(tt *testing.T){
    done := make(chan struct{})
    queue := NewQueue(0)
    go func() {
      queue.EnqueueNB(struct{}{})
      done <-struct{}{}
    }()
    select {
    case <-time.After(10 * time.Millisecond):
      queue.Close()
      tt.Errorf("non-blocking enqueue: queue1 reached capacity but non-blocking call")

    case <-done:
      tt.Log("non blocking ok1")
    }
  })

  t.Run("size1", func(tt *testing.T) {
    done  := make(chan struct{})
    queue := NewQueue(1)
    go func() {
      queue.EnqueueNB(struct{}{})
      done <-struct{}{}
    }()
    select {
    case <-time.After(10 * time.Millisecond):
      queue.Close()
      tt.Errorf("non-blocking enqueue: queue2 has free capacity")

    case <-done:
      tt.Log("non blocking ok2")
    }
  })
}

func TestBlockingEnqueueWithBlockingDequeue(t *testing.T) {
  done1  := make(chan struct{})
  done2  := make(chan struct{})

  value  := "pre value"
  queue  := NewQueue(0)

  go func() {
    <-done1
    v, ok := queue.Dequeue()
    if ok != true {
      t.Errorf("blocking dequeue w/o close")
    }
    value = v.(string)
    done2 <-struct{}{}
  }()
  <-time.After(10 * time.Millisecond)
  if value != "pre value" {
    t.Errorf("not run")
  }
  done1 <-struct{}{}
  <-time.After(10 * time.Millisecond)
  if value != "pre value" {
    t.Errorf("running but blocking")
  }
  queue.Enqueue("hello world")
  <-done2
  if value != "hello world" {
    t.Errorf("rendezvous blocking w/ enqueue+dequeue")
  }
}

func TestRecoveryHandlerEnqueue(t *testing.T) {
  done  := make(chan struct{})
  value := "not panic run"
  queue := NewQueue(0,
    QueuePanicHandler(func(pt PanicType, err interface{}) {
      if pt != PanicTypeEnqueue {
        t.Errorf("not enqueue panic %v", pt)
      }
      switch err.(type) {
      case error:
        value = "ok recover() handling"
      default:
        value = "not error type"
      }
    }),
  )

  go func() {
    queue.Enqueue(struct{}{})
    done <-struct{}{}
  }()

  select {
  case <-time.After(10 * time.Millisecond):
    // sending queue but close(channel) call
    queue.Close()

  case <-done:
    t.Errorf("not blocking")
  }
  <-time.After(10 * time.Millisecond)
  if value == "not panic run" {
    t.Errorf("PanicHandler does not call(1)")
  }
}
func TestRecoveryHandlerSendByClosedChannel(t *testing.T) {
  value := "not panic run"
  queue := NewQueue(0,
    QueuePanicHandler(func(pt PanicType, err interface{}) {
      if pt != PanicTypeEnqueue {
        t.Errorf("not enqueue panic %v", pt)
      }
      switch err.(type) {
      case error:
        // panic: send on closed channel
        value = "ok recover() handling"
      default:
        value = "not error type"
      }
    }),
  )

  queue.Close()
  queue.Enqueue(struct{}{})

  <-time.After(10 * time.Millisecond)

  if value == "not panic run" {
    t.Errorf("PanicHandler does not call(1)")
  }
}

func TestRecoveryHandlerWithDoubleClose(t *testing.T) {
  qv := "not double close"
  qc := NewQueue(0,
    QueuePanicHandler(func(pt PanicType, err interface{}) {
      if pt != PanicTypeClose {
        t.Errorf("not close panic %v", pt)
      }
      switch err.(type) {
      case error:
        // panic: close of closed channel
        qv = "ok double close chan handling"
      default:
        qv = "not error type"
      }
    }),
  )
  qc.Close()
  <-time.After(10 * time.Millisecond)
  if qv != "not double close" {
    t.Errorf("1st close is ok")
  }
  qc.Close()
  <-time.After(10 * time.Millisecond)
  if qv != "ok double close chan handling" {
    t.Errorf("PanicHandler does not call(2)")
  }
}

func TestEnqueueRetry(t *testing.T) {
  t.Run("NoReader", func(tt *testing.T) {
    q := NewQueue(0)
    write := q.EnqueueRetry("hello world", 1 * time.Millisecond, 10)
    if write {
      tt.Errorf("queue has no reader => expect missing enqueue")
    }
  })
  t.Run("DelayReader", func(tt *testing.T) {
    q := NewQueue(0)

    latch := make(chan struct{})
    enq := make(chan bool)
    go func(c chan struct{}, r chan bool){
      <-c
      r <-q.EnqueueRetry("hello world2", 10 * time.Millisecond, 10)
    }(latch, enq)

    type pair struct {
      val interface{}
      ok  bool
    }
    deq := make(chan pair)
    go func(c chan struct{}, r chan pair) {
      c <-struct{}{} // start latch

      time.Sleep(12 * time.Millisecond) // enqueue retry

      val, ok := q.Dequeue()
      r <-pair{val, ok}
    }(latch, deq)

    write := <-enq
    p := <-deq

    if write != true {
      tt.Errorf("enqueue should be succeed")
    }
    if p.ok != true {
      tt.Errorf("dequeue should be succeed")
    }
    if val, ok := p.val.(string); ok {
      if val != "hello world2" {
        tt.Errorf("value != %s", val)
      }
    } else {
      tt.Errorf("not string value %v", p.val)
    }
  })
}
func TestDequeueRetry(t *testing.T) {
  t.Run("NoWriter", func(tt *testing.T) {
    q := NewQueue(0)
    _, ok := q.DequeueRetry(1 * time.Millisecond, 10)
    if ok {
      t.Errorf("queue has no writer => expect missing dequeue")
    }
  })
  t.Run("DelayWriter", func(tt *testing.T) {
    q := NewQueue(0)

    type pair struct {
      val interface{}
      ok  bool
    }
    latch := make(chan struct{})
    deq   := make(chan pair)
    go func(c chan struct{}, r chan pair) {
      <-c
      val, ok := q.DequeueRetry(10 * time.Millisecond, 10)
      r <-pair{val, ok}
    }(latch, deq)

    enq := make(chan bool)
    go func(c chan struct{}, r chan bool) {
      c <-struct{}{} // start latch

      time.Sleep(12 * time.Millisecond) // dequeue retry

      r <-q.Enqueue("hello world3")
    }(latch, enq)

    p := <-deq
    write := <-enq

    if p.ok != true {
      tt.Errorf("dequeue should be succeed")
    }
    if val, ok := p.val.(string); ok {
      if val != "hello world3" {
        tt.Errorf("value != %s", val)
      }
    } else {
      tt.Errorf("not string value %v", p.val)
    }
    if write != true {
      tt.Errorf("enqueue should be succeed")
    }
  })
}
