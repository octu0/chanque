package chanque

import (
	"testing"
)

func TestPanicTypeString(t *testing.T) {
	type tuple struct {
		pt     PanicType
		expect string
	}
	tuples := []tuple{
		tuple{PanicTypeEnqueue, "enqueue"},
		tuple{PanicTypeDequeue, "dequeue"},
		tuple{PanicTypeClose, "close"},
		tuple{PanicType(0), "otherwise"},
		tuple{PanicType(255), "otherwise"},
	}
	for _, tuple := range tuples {
		if tuple.pt.String() != tuple.expect {
			t.Errorf("expect:%s actual %s", tuple.expect, tuple.pt)
		}
	}
}
