package pubsub

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testMsg struct {
	ID      int64
	Msg     string
	private string
}

func TestPublish(t *testing.T) {
	const topic = "topic1"
	const msg = "msg1"

	ps := NewPubSub[testMsg]()

	var wg sync.WaitGroup
	wg.Add(3)
	for i := 0; i < 3; i++ {
		go func() {
			defer wg.Done()
	
