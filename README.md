# `chanque`

[![GoDoc](https://godoc.org/github.com/octu0/chanque?status.svg)](https://godoc.org/github.com/octu0/chanque)

`chanque` is simple go channel utility

## Installation

```
$ go get github.com/octu0/chanque
```

## Usage

Import the package

```go
import(
  "fmt"
  "github.com/octu0/chanque"
)

func main(){
  que1 := chanque.New(10)
  defer que1.Close()

  go func(){
    for {
      val := que1.Dequeue()
      fmt.Println(val.(string))
    }
  }()
  if ok := que1.Enqueue("hello"); ok {
    fmt.Println("enqueue-ed")
  }

  que2 := chanque.New(10)
  que2.PanicHandler(func(rcv interface{}, ret *bool) {
    switch err.(type) {
    case error:
      fmt.Println("panic occurred")
    }
  })
  if ok := que2.EnqueueNB("world w/ non-blocking enqueue"); ok {
    fmt.Println("enqueue-ed")
  }
}
```

### Functions

```
New(capacity int) *Queue

Enqueue(value interface{}) (written bool)
EnqueueNB(value interface{}) (written bool)
EnqueueRetry(value interface{}, interval time.Duration, retry int) (written bool)

Dequeue() (value interface{}, found bool)
DequeueNB() (value interface{}, found bool)
DequeueRetry(interval time.Duration, retry int) (value interface{}, found bool)
```
