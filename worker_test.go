package chanque

import(
  "testing"
  "fmt"
  "time"
)

func TestBufferWorkerShutdownAndWait(t *testing.T) {
  t.Run("1-1", func(tt *testing.T) {
    h := func(p interface{}) {
      time.Sleep(10 * time.Millisecond)
      fmt.Printf("val = %v\n", p)
    }
    w := NewBufferWorker(h, 1, 1)
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
    w := NewBufferWorker(h, 1, 5)
    w.Run(nil)
    for i := 0; i < 10; i += 1 {
      w.Enqueue(i)
    }
    w.ShutdownAndWait()
  })
}
