package pubsub

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// Contains and manage the map of topics -> subscribers
var topicsSubs struct {
	sync.Mutex
	m map[string][]*Sub
}

func init() {
	topicsSubs.m = make(map[string][]*Sub)
}

func getSubscribers(topic string) []*Sub {
	topicsSubs.Lock()
	defer topicsSubs.Unlock()
	return topicsSubs.m[topic]
}

func addSubscriber(s *Sub) {
	topicsSubs.Lock()
	for _, topic := range s.topics {
		topicsSubs.m[topic] = append(topicsSubs.m[topic], s)
	}
	topicsSubs.Unlock()
}

func removeSubscriber(s *Sub) {
	topicsSubs.Lock()
	for _, topic := range s.topics {
		for i, subscriber := range topicsSubs.m[topic] {
			if subscriber == s {
				topicsSubs.m[topic] = append(topicsSubs.m[topic][:i], topicsSubs.m[topic][i+1:]...)
				break
			}
		}
	}
	topicsSubs.Unlock()
}

//
type payload struct {
	topic string
	msg   string
}

// ErrTimeout error returned when timeout occurs
var ErrTimeout = errors.New("timeout")

// ErrCancelled error returned when context is cancelled
var ErrCancelled = errors.New("cancelled")

// Sub subscriber will receive messages published on a topic in his ch
type Sub struct {
	topics []string     // Topics subscribed to
	ch     chan payload // Receives messages in this channel
	ctx    context.Context
	cancel context.CancelFunc
}

// NewSub creates a new subscriber for topics
func NewSub(topics []string) *Sub {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Sub{topics: topics, ch: make(chan payload, 10), ctx: ctx, cancel: cancel}
	addSubscriber(s)
	return s
}

// ReceiveTimeout returns a message received on the channel or timeout
func (s *Sub) ReceiveTimeout(timeout time.Duration) (topic string, msg string, err error) {
	select {
	case p := <-s.ch:
		return p.topic, p.msg, nil
	case <-time.After(timeout):
		return topic, msg, ErrTimeout
	case <-s.ctx.Done():
		return topic, msg, ErrCancelled
	}
}

// Receive returns a message
func (s *Sub) Receive() (topic string, msg string, err error) {
	var res string
	select {
	case p := <-s.ch:
		return p.topic, p.msg, nil
	case <-s.ctx.Done():
		return topic, res, ErrCancelled
	}
}

// Close will remove the subscriber from the topic subscribers
func (s *Sub) Close() {
	s.cancel()
	removeSubscriber(s)
}

// publish a message to the subscriber channel
func (s *Sub) publish(p payload) {
	select {
	case s.ch <- p:
	default:
	}
}

// Subscribe is an alias for NewSub
func Subscribe(topics []string) *Sub {
	return NewSub(topics)
}

// PublishString a message to all subscribers of a topic
func PublishString(topic string, msg string) {
	for _, s := range getSubscribers(topic) {
		s.publish(payload{topic, msg})
	}
}

// Publish a message to all subscribers of a topic
func Publish(topic string, msg any) error {
	marshalled, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	PublishString(topic, string(marshalled))
	return nil
}

// Pub shortcut for publish which ignore the error
func Pub(topic string, msg any) {
	_ = Publish(topic, msg)
}
