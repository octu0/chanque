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
  if ok := que2.EnqueueNB("world w/ non-blocking enqueue"); ok {
    fmt.Println("enqueue-ed")
  }
}
```

### Functions

```
Enqueue(interface{})
EnqueueNB(interface{})
EnqueueRetry(interface{}, time.Duration, int)

Dequeue() interface{}
DequeueNB() (interface{}, bool)
DequeueRetry(time.Duration, int) (interface{}, bool)
```
