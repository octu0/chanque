package chanque

import(
  "testing"
  "time"
  "math/rand"
  "sync"
  "sync/atomic"
)

func TestContextSimple(t *testing.T) {
  rand.Seed(time.Now().UnixNano())

  heavy := func(id int) error {
    time.Sleep(time.Duration(rand.Intn(10) + id) * time.Millisecond)
    t.Logf("process %d", id)
    return nil
  }
  dispatcher := func(id int, done func()) Job {
    return func() {
      defer done()
      if err := heavy(id); err != nil {
        t.Errorf("error occurred %v", err)
      }
    }
  }

  e   := NewExecutor(10, 10)
  defer e.Release()

  val := "hello world"
  ctx := NewContext(e, func(){
    val = "all done"
  })
  for i := 0; i < 10; i += 1 {
    e.Submit(dispatcher(i, ctx.Add()))
  }
  ctx.Background()

  time.Sleep(100 * time.Millisecond)

  if val == "hello world" {
    t.Errorf("not all done")
  }
}

func TestContextTimeout(t *testing.T) {
  f := func(workerTimeout time.Duration, tt *testing.T, preAssert, postAssert func(int32)) {
    e := NewExecutor(10, 10)
    defer e.Release()

    val  := "hello world"
    done := func(){
      val = "all done"
      tt.Logf("all done")
    }
    ctx  := NewContextTimeout(e, done, 50 * time.Millisecond)
    run  := int32(0)
    wake := new(sync.WaitGroup)
    for i := 0; i < 10; i += 1 {
      wake.Add(1)
      go func(w *sync.WaitGroup, id int, done func()) {
        defer done()
        atomic.AddInt32(&run, 1)
        defer atomic.AddInt32(&run, -1)

        w.Done()
        tt.Logf("worker running %d", id)
        time.Sleep(workerTimeout)
        tt.Logf("worker done %d", id)
      }(wake, i, ctx.Add())
    }
    wake.Wait()

    r1 := atomic.LoadInt32(&run)
    preAssert(r1)

    tn := time.Now()
    ctx.Wait()
    tt.Logf("wait time = %s", time.Since(tn))

    r2 := atomic.LoadInt32(&run)
    postAssert(r2)

    if val == "hello world" {
      t.Errorf("workers not all done but cancel happen")
    }

    time.Sleep(150 * time.Millisecond)

    r3 := atomic.LoadInt32(&run)
    if r3 != 0 {
      tt.Errorf("workers all done 0 != %d", r3)
    }
  }
  t.Run("worker-timeout", func(tt *testing.T) {
    f(100 * time.Millisecond, tt, func(run int32){
      if run != 10 {
        tt.Errorf("workers not done 10 != %d", run)
      }
    }, func(run int32){
      if run != 10 {
        tt.Errorf("workers not all done 10 != %d", run)
      }
    })
  })

  t.Run("worker-done", func(tt *testing.T) {
    f(10 * time.Millisecond, tt, func(run int32){
      if run != 10 {
        tt.Errorf("workers not done 10 != %d", run)
      }
    }, func(run int32){
      if run != 0 {
        tt.Errorf("workers already all done 0 != %d", run)
      }
    })
  })
}
