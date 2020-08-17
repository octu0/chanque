package chanque

import (
	"context"
	"time"
)

type DoneFunc func()

type Context struct {
	waits *SubExecutor
	bg    *SubExecutor
	done  DoneFunc
}

func NewContext(executor *Executor, done DoneFunc) *Context {
	c := new(Context)
	c.waits = executor.SubExecutor()
	c.bg = executor.SubExecutor()
	c.done = done
	return c
}

func (c *Context) createWriteDone(done chan struct{}) func() {
	return func() {
		done <- struct{}{}
	}
}

func (c *Context) createReadDone(done chan struct{}) Job {
	return func() {
		<-done
	}
}

func (c *Context) Add() func() {
	done := make(chan struct{})
	f := c.createWriteDone(done)
	c.waits.Submit(c.createReadDone(done))
	return f
}

func (c *Context) Wait() {
	c.waits.Wait()
	c.done()
}

func (c *Context) Background() {
	c.bg.Submit(c.Wait)
}

type ContextTimeout struct {
	ctx     context.Context
	cancel  context.CancelFunc
	timeout time.Duration
	waits   *SubExecutor
	bg      *SubExecutor
	done    DoneFunc
}

func NewContextTimeout(executor *Executor, done DoneFunc, timeout time.Duration) *ContextTimeout {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	c := new(ContextTimeout)
	c.ctx = ctx
	c.cancel = cancel
	c.timeout = timeout
	c.waits = executor.SubExecutor()
	c.bg = executor.SubExecutor()
	c.done = done
	return c
}
func (c *ContextTimeout) createWaitContextDone(ctx context.Context, cancel context.CancelFunc) Job {
	return func() {
		defer cancel()
		<-ctx.Done()
	}
}
func (c *ContextTimeout) Add() func() {
	ctx, cancel := context.WithTimeout(c.ctx, c.timeout)
	c.waits.Submit(c.createWaitContextDone(ctx, cancel))
	return cancel
}
func (c *ContextTimeout) Wait() {
	defer c.cancel()

	c.waits.Wait()
	<-c.ctx.Done()
	c.done()
}
func (c *ContextTimeout) Background() {
	c.bg.Submit(c.Wait)
}
