package chanque

import(
  "encoding/binary"
  "io"
  "math"
  "math/rand"
  "sync/atomic"
  "time"

  cryptoRnd "crypto/rand"
)

const (
  maxInt64 = int(math.MaxInt64)
)

var(
  RetryRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func init() {
  buf := make([]byte, 8)
  _, err := io.ReadFull(cryptoRnd.Reader, buf[:])
  if err != nil {
    return
  }
  n := int64(binary.LittleEndian.Uint64(buf[:]))
  RetryRand = rand.New(rand.NewSource(n))
}

type Backoff struct {
  min     time.Duration
  max     time.Duration
  jitter  bool
  attempt uint64
}

func NewBackoff(min, max time.Duration) *Backoff {
  return newBackoff(min, max, true)
}

func NewBackoffNoJitter(min, max time.Duration) *Backoff {
  return newBackoff(min, max, false)
}

func newBackoff(min, max time.Duration, useJitter bool) *Backoff {
  if min < 1 {
    min = 1
  }
  if max < 1 {
    max = 1
  }
  if max < min {
    max = min
  }

  return &Backoff{
    min:     min,
    max:     max,
    jitter:  useJitter,
    attempt: 0,
  }
}

func (b *Backoff) Next() time.Duration {
  n := atomic.AddUint64(&b.attempt, 1) - 1
  pow := 1 << n // math.Pow(2, n). binary exponential
  if  maxInt64 <= pow {
    pow = maxInt64 - 1
  }

  powDur := time.Duration(pow)
  dur    := b.min * powDur
  if dur <= 0 {
    dur = math.MaxInt64 // overflow
  }
  if b.jitter {
    size := int64(dur - b.min)
    if 0 < size {
      // must 1 < attempt
      rnd := RetryRand.Int63n(size)
      dur += time.Duration(rnd)
      if dur <= 0 {
        dur = math.MaxInt64 // overflow
      }
    }
  }

  if b.max <= dur {
    dur = b.max // truncate
  }
  return dur
}

func (b *Backoff) Reset() {
  atomic.StoreUint64(&b.attempt, 0)
}
