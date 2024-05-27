package pubsub

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// PubSub contains and manage the map of topics -> subscribers
type PubSub[T any] struct {
	sync.Mutex
	m map[string][]*Sub[T]
}

func NewPubSub[T any]() *PubSub[T] {
	ps := PubSub[T]{}
	ps.m = make(map[string][]*Sub[T])
	return &ps
}

func (p *PubSub[T]) getSubscribers(topic string) []*Sub[T] {
	p.Lock()
	defer p.Unlock()
	return p.m[topic]
}

func (p *PubSub[T]) addSubscriber(s *Sub[T]) {
	p.Lock()
	for _, topic := range s.topics {
		p.m[topic] = append(p.m[topic], s)
	}
	p.Unlock()
}

func (p *PubSub[T]) removeSubscriber(s *Sub[T]) {
	p.Lock()
	for _, topic := range s.topics {
		for i, subscriber := range p.m[topic] {
			if subscriber == s {
				p.m[topic] = append(p.m[topic][:i], p.m[topic][i+1:]...)
				break
			}
		}
	}
	p.Unlock()
}

// Subscribe is an alias for NewSub
func (p *PubSub[T]) Subscribe(topics []string) *Sub[T] {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Sub[T]{topics: topics, ch: make(chan Payload[T], 10), ctx: ctx, cancel: cancel, p: p}
	p.addSubscriber(s)
	return s
}

// Pub shortcut for publish which ignore the error
func (p *PubSub[T]) Pub(topic string, msg T) {
	for _, s := range p.getSubscribers(topic) {
		s.publish(Payload[T]{topic, msg})
	}
}

type Payload[T any] struct {
	Topic string
	Msg   T
}

// ErrTimeout error returned when timeout occurs
var ErrTimeout = errors.New("timeout")

// ErrCancelled error returned when context is cancelled
var ErrCancelled = errors.New("cancelled")

// Sub subscriber will receive messages published on a Topic in his ch
type Sub[T any] struct {
	topics []string        // Topics subscribed to
	ch     chan Payload[T] // Receives messages in this channel
	ctx    context.Context
	cancel context.CancelFunc
	p      *PubSub[T]
}

// ReceiveTimeout2 returns a message received on the channel or timeout
func (s *Sub[T]) ReceiveTimeout2(timeout time.Duration, c1 <-chan struct{}) (topic string, msg T, err error) {
	select {
	case p := <-s.ch:
		return p.Topic, p.Msg, nil
	case <-time.After(timeout):
		return topic, msg, ErrTimeout
	case <-c1:
		return topic, msg, ErrCancelled
	case <-s.ctx.Done():
		return topic, msg, ErrCancelled
	}
}

// ReceiveTimeout returns a message received on the channel or timeout
func (s *Sub[T]) ReceiveTimeout(timeout time.Duration) (topic string, msg T, err error) {
	c1 := make(chan struct{})
	return s.ReceiveTimeout2(timeout, c1)
}

// Receive returns a message
func (s *Sub[T]) Receive() (topic string, msg T, err error) {
	var res T
	select {
	case p := <-s.ch:
		return p.Topic, p.Msg, nil
	case <-s.ctx.Done():
		return topic, res, ErrCancelled
	}
}

// ReceiveCh returns a message
func (s *Sub[T]) ReceiveCh() <-chan Payload[T] {
	return s.ch
}

// Close will remove the subscriber from the Topic subscribers
func (s *Sub[T]) Close() {
	s.cancel()
	s.p.removeSubscriber(s)
}

// publish a message to the subscriber channel
func (s *Sub[T]) publish(p Payload[T]) {
	select {
	case s.ch <- p:
	default:
	}
}
