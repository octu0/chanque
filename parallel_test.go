package chanque

import (
	"errors"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestParallelResult(t *testing.T) {
	e := NewExecutor(10, 10)
	defer e.Release()

	p := NewParallel(e)
	p.Queue(func() (interface{}, error) {
		return "#1 result", nil
	})
	p.Queue(func() (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return "#2 result", nil
	})
	p.Queue(func() (interface{}, error) {
		time.Sleep(50 * time.Millisecond)
		return nil, errors.New("#3 error")
	})

	// order in which process was completed
	expect := []*tupleValueError{
		&tupleValueError{"#1 result", nil},
		&tupleValueError{nil, errors.New("#3 error")},
		&tupleValueError{"#2 result", nil},
	}

	vals := make([]string, 0)
	errors := make([]error, 0)
	f := p.Submit()
	for i, r := range f.Result() {
		if expect[i].value != r.Value() {
			t.Errorf("expect:%v actual:%v", expect[i].value, r.Value())
		}
		if expect[i].err != nil {
			if r.Err() == nil {
				t.Errorf("expect:%v actual:nil", expect[i].err)
			}
			if expect[i].err.Error() != r.Err().Error() {
				t.Errorf("expect:'%v' actual:'%v'", expect[i].err, r.Err())
			}
		}
		if r.Value() != nil {
			vals = append(vals, r.Value().(string))
		}
		if r.Err() != nil {
			errors = append(errors, r.Err())
		}
	}

	s := strings.Join(vals, ",")
	t.Log(s)
	if "#1 result,#2 result" != s {
		t.Errorf("string value mismatch: %s", s)
	}
	if len(errors) != 1 {
		t.Errorf("error one")
	}
	t.Log(errors[0].Error())
	if errors[0].Error() != "#3 error" {
		t.Errorf("error message mismatch: %s", errors[0].Error())
	}
}
func TestParallelParallelism(t *testing.T) {
	t.Run("p1", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		p := NewParallel(e, Parallelism(1))
		p.Queue(func() (interface{}, error) {
			time.Sleep(100 * time.Millisecond)
			return "v1", nil
		})
		p.Queue(func() (interface{}, error) {
			return "v2", nil
		})
		p.Queue(func() (interface{}, error) {
			time.Sleep(50 * time.Millisecond)
			return "v3", nil
		})

		f := p.Submit()
		s := make([]string, 0)
		for _, r := range f.Result() {
			s = append(s, r.Value().(string))
		}
		if "v1,v2,v3" != strings.Join(s, ",") {
			tt.Errorf("parallelism 1 %v", s)
		}
	})
	t.Run("p2", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		p := NewParallel(e, Parallelism(2))
		p.Queue(func() (interface{}, error) {
			time.Sleep(100 * time.Millisecond)
			return "v1", nil
		})
		p.Queue(func() (interface{}, error) {
			return "v2", nil
		})
		p.Queue(func() (interface{}, error) {
			time.Sleep(50 * time.Millisecond)
			return "v3", nil
		})

		f := p.Submit()
		s := make([]string, 0)
		for _, r := range f.Result() {
			s = append(s, r.Value().(string))
		}
		if "v2,v1,v3" != strings.Join(s, ",") {
			tt.Errorf("parallelism 2 %v", s)
		}
	})
	t.Run("p4", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		p := NewParallel(e, Parallelism(4))
		p.Queue(func() (interface{}, error) {
			time.Sleep(100 * time.Millisecond)
			return "v1", nil
		})
		p.Queue(func() (interface{}, error) {
			return "v2", nil
		})
		p.Queue(func() (interface{}, error) {
			time.Sleep(50 * time.Millisecond)
			return "v3", nil
		})

		f := p.Submit()
		s := make([]string, 0)
		for _, r := range f.Result() {
			s = append(s, r.Value().(string))
		}
		if "v2,v3,v1" != strings.Join(s, ",") {
			tt.Errorf("parallelism 4 %v", s)
		}
	})
}

func TestParallelErrors(t *testing.T) {
	t.Run("double-call-Submit", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		p := NewParallel(e)
		p.Queue(func() (interface{}, error) {
			return "v1", nil
		})
		p.Queue(func() (interface{}, error) {
			return "v2", nil
		})

		f1 := p.Submit()
		f2 := p.Submit()

		if len(f1.Result()) != 2 {
			tt.Errorf("submit 2 queue")
		}
		if len(f2.Result()) != 0 {
			tt.Errorf("submit clear old queue")
		}
	})
	t.Run("double-call-Result", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		p := NewParallel(e)
		p.Queue(func() (interface{}, error) {
			return "v1", nil
		})
		p.Queue(func() (interface{}, error) {
			return "v2", nil
		})

		f := p.Submit()
		r1 := f.Result()
		r2 := f.Result()
		if len(r1) != 2 {
			tt.Errorf("2 result")
		}
		if len(r2) != 2 {
			tt.Errorf("2 result")
		}
		for i, _ := range r1 {
			if r1[i].Value().(string) != r2[i].Value().(string) {
				tt.Errorf("expect same result")
			}
		}
	})
	t.Run("reuse-submit", func(tt *testing.T) {
		e := NewExecutor(10, 10)
		defer e.Release()

		p := NewParallel(e)
		p.Queue(func() (interface{}, error) {
			return "v1", nil
		})
		p.Queue(func() (interface{}, error) {
			return "v2", nil
		})
		f1 := p.Submit()

		p.Queue(func() (interface{}, error) {
			return "v3", nil
		})
		f2 := p.Submit()

		p.Queue(func() (interface{}, error) {
			return "v4", nil
		})
		p.Queue(func() (interface{}, error) {
			return "v5", nil
		})
		p.Queue(func() (interface{}, error) {
			return "v6", nil
		})
		f3 := p.Submit()

		r1 := f1.Result()
		r2 := f2.Result()
		r3 := f3.Result()

		s1 := make([]string, 0)
		s2 := make([]string, 0)
		s3 := make([]string, 0)
		for _, r := range r1 {
			s1 = append(s1, r.Value().(string))
		}
		for _, r := range r2 {
			s2 = append(s2, r.Value().(string))
		}
		for _, r := range r3 {
			s3 = append(s3, r.Value().(string))
		}
		sort.Strings(s1)
		sort.Strings(s2)
		sort.Strings(s3)
		if "v1,v2" != strings.Join(s1, ",") {
			tt.Errorf("#1 result 1,2 actual:%s", s1)
		}
		if "v3" != strings.Join(s2, ",") {
			tt.Errorf("#2 result 3 actual:%s", s2)
		}
		if "v4,v5,v6" != strings.Join(s3, ",") {
			tt.Errorf("#2 result 4,5,6 actual:%s", s3)
		}
	})
}
