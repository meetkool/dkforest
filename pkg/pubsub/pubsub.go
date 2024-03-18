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
	subscribers := p.m[topic]
	return subscribers
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
	
