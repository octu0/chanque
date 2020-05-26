package chanque

import(
  "testing"
  "time"
  "math/rand"
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

  e   := NewExecutor(1, 10)
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
