package chanque

import(
  "testing"
  "time"
  "fmt"
  "strings"
)

func TestPipelineInOutParameter(t *testing.T) {
  type Bar struct {
    quux int
  }
  type Foo struct {
    foo  string
    bar  *Bar
  }
  type tuple struct {
    name    string
    in      func(*testing.T) PipelineInputFunc
    out     func(*testing.T) PipelineOutputFunc
    enq     func(*testing.T, *Pipeline)
  }
  tuples := []tuple{
    tuple{
      name: "string",
      in: func(t *testing.T) PipelineInputFunc {
        return func(param interface{}) (interface{}, error) {
          time.Sleep(10 * time.Millisecond)
          if val, ok := param.(string); ok {
            return strings.Join([]string{val, "@", val}, ""), nil
          }
          return nil, fmt.Errorf("invalid param")
        }
      },
      out: func(t *testing.T) PipelineOutputFunc {
        return func(result interface{}, err error) {
          if err != nil {
            t.Errorf("%w", err)
          }
          val, ok := result.(string)
          if ok != true {
            t.Errorf("invalid result : %v", result)
          }
          t.Log("out ok", val)
        }
      },
      enq: func(t *testing.T, p *Pipeline){
        p.Enqueue("hello")
        p.Enqueue("world")
        p.ShutdownAndWait()
      },
    },
    tuple{
      name: "int",
      in: func(t *testing.T) PipelineInputFunc {
        return func(param interface{}) (interface{}, error) {
          time.Sleep(10 * time.Millisecond)
          if val, ok := param.(int); ok {
            return val + 1, nil
          }
          return nil, fmt.Errorf("invalid param")
        }
      },
      out: func(t *testing.T) PipelineOutputFunc {
        return func(result interface{}, err error) {
          if err != nil {
            t.Errorf("%w", err)
          }
          val, ok := result.(int)
          if ok != true {
            t.Errorf("invalid result : %v", result)
          }
          t.Log("out ok", val)
        }
      },
      enq: func(t *testing.T, p *Pipeline){
        p.Enqueue(10)
        p.Enqueue(20)
        p.Enqueue(30)
        p.Enqueue(40)
        p.Enqueue(50)
        p.ShutdownAndWait()
      },
    },
    tuple{
      name: "struct",
      in: func(t *testing.T) PipelineInputFunc {
        return func(param interface{}) (interface{}, error) {
          time.Sleep(10 * time.Millisecond)
          if val, ok := param.(*Foo); ok {
            val.bar = &Bar{123}
            return val, nil
          }
          return nil, fmt.Errorf("invalid param")
        }
      },
      out: func(t *testing.T) PipelineOutputFunc {
        return func(result interface{}, err error) {
          if err != nil {
            t.Errorf("%w", err)
          }
          val, ok := result.(*Foo)
          if ok != true {
            t.Errorf("invalid result : %v", result)
          }
          t.Log("foo:", val.foo, "bar:", val.bar.quux)
        }
      },
      enq: func(t *testing.T, p *Pipeline){
        p.Enqueue(&Foo{
          foo: "i am foo",
        })
        p.ShutdownAndWait()
      },
    },
  }

  for _, tuple := range tuples {
    t.Run(tuple.name, func(tt *testing.T) {
      p := CreatePipeline(1, 10, tuple.in(tt), tuple.out(tt),
        PipelinePanicHandler(func(a PanicType, b interface{}){
          /* nopp */
        }),
      )
      tuple.enq(tt, p)
    })
  }
}
