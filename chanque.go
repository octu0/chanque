package chanque

type Queue struct {
  ch chan interface{}
}

func New(c int) *Queue {
  return &Queue{make(chan interface{}, c)}
}

// blocking enqueue
func (q *Queue) Enqueue(val interface{}) {
  q.ch <-val
}

// non-blocking enqueue
func (q *Queue) EnqueueNB(val interface{}) (write bool) {
  select {
  case q.ch <-val:
    write = true
  default:
    // drop
    write = false
  }
  return
}

// retry w/ enqueue until channel can be written(waiting for channel to read)
func (q *Queue) EnqueueRetry(val interface{}, retryInterval time.Duration, retryLimit int) (timeout bool) {
  for i := 0; i < retryLimit; i += 1 {
    if q.EnqueueNB(val) {
      timeout = false
      return
    }
    time.Sleep(retryInterval)
  }
  timeout = true
  return
}

// blocking dequeue
func (q *Queue) Dequeue() interface{} {
  return <-q.ch
}

// non-blocking dequeue
func (q *Queue) DequeueNB() (val interface{}, found bool) {
  select {
  case v = <-q.ch:
    val, found = v, true
  default:
    val, found = nil, false
  }
  return
}

// retry w/ dequeue until channel can be read(waiting for channel to write)
func (q *Queue) DequeueRetry(retryInterval time.Duration, retryLimit int) (val interface{}, found bool) {
  for i := 0; i < retryLimit; i += 1 {
    if v, ok := q.DequeueNB(); ok {
      val, found = v, true
      return
    }
    time.Sleep(retryInterval)
  }
  val, found = nil, false
  return
}
