package ratecounter

import (
	"sync/atomic"
	"time"
)

type RateCounter struct {
	currTime int64
	prevTime int64
	curr     int64
	prev     int64
}

func NewRateCounter() *RateCounter {
	return &RateCounter{}
}

func (r *RateCounter) Incr() {
	now := time.Now().Unix()
	currTime := atomic.LoadInt64(&r.currTime)
	if currTime == now {
		atomic.AddInt64(&r.curr, 1)
		return
	}
	curr := atomic.LoadInt64(&r.curr)
	atomic.StoreInt64(&r.prevTime, currTime)
	atomic.StoreInt64(&r.prev, curr)
	atomic.StoreInt64(&r.curr, 1)
	atomic.StoreInt64(&r.currTime, now)
}

func (r *RateCounter) Rate() int64 {
	n := time.Now().Unix()
	if atomic.LoadInt64(&r.prevTime) == n-1 {
		return atomic.LoadInt64(&r.prev)
	}
	return 0
}
