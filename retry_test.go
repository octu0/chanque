package chanque

import(
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
    b := NewBackoffNoJitter(100 * time.Millisecond, 3 * time.Second)
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
    eqDur(tt, 100 * time.Millisecond, n1)
    eqDur(tt, 200 * time.Millisecond, n2)
    eqDur(tt, 400 * time.Millisecond, n3)
    eqDur(tt, 800 * time.Millisecond, n4)
    eqDur(tt, 1600 * time.Millisecond, n5)
    eqDur(tt, 3000 * time.Millisecond, n6) // truncate max
    eqDur(tt, 3000 * time.Millisecond, n7)
    eqDur(tt, 3000 * time.Millisecond, n8)
    eqDur(tt, 3000 * time.Millisecond, n9)
    eqDur(tt, 3000 * time.Millisecond, n10)
  })
  t.Run("min=100ms/max=3s/Jitter", func(tt *testing.T) {
    b := NewBackoff(100 * time.Millisecond, 3 * time.Second)
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
    eqDur(tt, 100 * time.Millisecond, n1) // first = no jitter
    rangeDur(tt, 200 * time.Millisecond, n3, n2)
    rangeDur(tt, 400 * time.Millisecond, n4, n3)
    rangeDur(tt, 800 * time.Millisecond, n5, n4)
    rangeDur(tt, 1600 * time.Millisecond, n6, n5)
    eqDur(tt, 3000 * time.Millisecond, n6) // truncate
    eqDur(tt, 3000 * time.Millisecond, n7)
    eqDur(tt, 3000 * time.Millisecond, n8)
    eqDur(tt, 3000 * time.Millisecond, n9)
    eqDur(tt, 3000 * time.Millisecond, n10)
  })
}
