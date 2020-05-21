package chanque

import(
  "testing"
  "time"
)

func TestBlockingEnqueue(t *testing.T) {
  done1  := make(chan struct{})
  queue1 := NewQueue(0)

  go func(){
    queue1.Enqueue(struct{}{})
    done1 <-struct{}{}
  }()

  select {
  case <-time.After(10 * time.Millisecond):
    println("blocking enqueue ok1")
    queue1.Close()

  case <-done1:
    t.Errorf("not blocking enqueue 1")
  }

  done2  := make(chan struct{})
  queue2 := NewQueue(1)

  go func(){
    queue2.Enqueue(struct{}{})
    done2 <-struct{}{}
  }()

  select {
  case <-time.After(10 * time.Millisecond):
    queue2.Close()
    t.Errorf("blocking enqueue: queue2 has free capacity")

  case <-done2:
    println("blocking enqueue ok2")
  }
}

func TestBlockingEnqueueNB(t *testing.T) {
  done1 := make(chan struct{})
  queue1 := NewQueue(0)
  go func() {
    queue1.EnqueueNB(struct{}{})
    done1 <-struct{}{}
  }()
  select {
  case <-time.After(10 * time.Millisecond):
    queue1.Close()
    t.Errorf("non-blocking enqueue: queue1 reached capacity but non-blocking call")

  case <-done1:
    println("non blocking ok1")
  }

  done2 := make(chan struct{})
  queue2 := NewQueue(1)
  go func() {
    queue2.EnqueueNB(struct{}{})
    done2 <-struct{}{}
  }()
  select {
  case <-time.After(10 * time.Millisecond):
    queue2.Close()
    t.Errorf("non-blocking enqueue: queue2 has free capacity")

  case <-done2:
    println("non blocking ok2")
  }
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
  queue := NewQueue(0)
  queue.PanicHandler(func(pt PanicType, err interface{}) {
    if pt != PanicTypeEnqueue {
      t.Errorf("not enqueue panic %v", pt)
    }
    switch err.(type) {
    case error:
      value = "ok recover() handling"
    default:
      value = "not error type"
    }
  })

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
  queue := NewQueue(0)
  queue.PanicHandler(func(pt PanicType, err interface{}) {
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
  })

  queue.Close()
  queue.Enqueue(struct{}{})

  <-time.After(10 * time.Millisecond)

  if value == "not panic run" {
    t.Errorf("PanicHandler does not call(1)")
  }
}

func TestRecoveryHandlerWithDoubleClose(t *testing.T) {
  qv := "not double close"
  qc := NewQueue(0)
  qc.PanicHandler(func(pt PanicType, err interface{}) {
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
  })
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
