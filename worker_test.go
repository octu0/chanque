package chanque

import(
  "testing"
  "time"
  "sync"
  "math/rand"
)

func TestWorkerSequence(t *testing.T) {
  t.Run("default", func(tt *testing.T) {
    rand.Seed(time.Now().UnixNano())

    c := make([]int, 0)
    s := 30
    m := new(sync.Mutex)
    h := func(p interface{}) {
      m.Lock()
      defer m.Unlock()

      c = append(c, p.(int))

      time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
    }
    w := NewDefaultWorker(h)
    w.Run(nil)
    for i := 0; i < s; i += 1 {
      w.Enqueue(i)
    }
    w.ShutdownAndWait()

    tt.Logf("v = %v", c)
    if len(c) != s {
      tt.Errorf("c should be len = %d", s)
    }
    for i := 0; i < s; i += 1 {
      if c[i] != i {
        tt.Errorf("not sequencial c[%d] != %d", i, c[i])
      }
    }
  })
  t.Run("buffer", func(tt *testing.T) {
    rand.Seed(time.Now().UnixNano())

    c := make([]int, 0)
    s := 30
    m := new(sync.Mutex)
    h := func(p interface{}) {
      m.Lock()
      defer m.Unlock()

      c = append(c, p.(int))

      time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
    }
    w := NewBufferWorker(h)
    w.Run(nil)
    for i := 0; i < s; i += 1 {
      w.Enqueue(i)
    }
    w.ShutdownAndWait()

    tt.Logf("v = %v", c)
    if len(c) != s {
      tt.Errorf("c should be len = %d", s)
    }
    for i := 0; i < s; i += 1 {
      if c[i] != i {
        tt.Errorf("not sequencial c[%d] != %d", i, c[i])
      }
    }
  })
}

func TestDefaultWorkerShutdownAndWait(t *testing.T) {
  t.Run("1-1", func(tt *testing.T) {
    h := func(p interface{}) {
      time.Sleep(10 * time.Millisecond)
      tt.Logf("val = %v", p)
    }
    w := NewDefaultWorker(h)
    w.Run(nil)
    w.Enqueue(10)
    w.Enqueue(20)
    w.Enqueue(30)
    w.ShutdownAndWait()
  })
  t.Run("1-5", func(tt *testing.T) {
    h := func(p interface{}) {
      time.Sleep(10 * time.Millisecond)
    }
    w := NewDefaultWorker(h)
    w.Run(nil)
    for i := 0; i < 10; i += 1 {
      w.Enqueue(i)
    }
    w.ShutdownAndWait()
  })
}

func TestBufferWorkerShutdownAndWait(t *testing.T) {
  t.Run("3", func(tt *testing.T) {
    h := func(p interface{}) {
      time.Sleep(10 * time.Millisecond)
      tt.Logf("val = %v", p)
    }
    w := NewBufferWorker(h)
    w.Run(nil)
    w.Enqueue(10)
    w.Enqueue(20)
    w.Enqueue(30)
    w.ShutdownAndWait()
  })
  t.Run("10", func(tt *testing.T) {
    h := func(p interface{}) {
      time.Sleep(10 * time.Millisecond)
    }
    w := NewBufferWorker(h)
    w.Run(nil)
    for i := 0; i < 10; i += 1 {
      w.Enqueue(i)
    }
    w.ShutdownAndWait()
  })
}
