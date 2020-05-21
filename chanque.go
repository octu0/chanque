package chanque

import(
  "log"
  "runtime/debug"
)

type PanicType uint8
const(
  PanicTypeEnqueue PanicType = iota + 1
  PanicTypeDequeue
  PanicTypeClose
)

func (p PanicType) String() string {
  switch p {
  case PanicTypeEnqueue:
    return "enqueue"
  case PanicTypeDequeue:
    return "dequeue"
  case PanicTypeClose:
    return "close"
  }
  return "otherwise"
}

type PanicHandler func(PanicType, interface{})
func defaultPanicHandler(pt PanicType, rcv interface{}) {
  log.Printf("warn: [recover] panic(%s) occurred %v stack %s", pt, rcv, string(debug.Stack()))
}
