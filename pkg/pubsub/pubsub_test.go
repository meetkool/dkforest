package pubsub

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPublish(t *testing.T) {
	topic := "topic1"
	msg := "msg1"

	s1 := Subscribe([]string{topic})
	s2 := Subscribe([]string{topic})
	s3 := Subscribe([]string{topic})

	PublishString(topic, msg)

	s1Topic, s1Msg, s1Err := s1.ReceiveTimeout(time.Second)
	s2Topic, s2Msg, s2Err := s2.ReceiveTimeout(time.Second)
	s3Topic, s3Msg, s3Err := s3.ReceiveTimeout(time.Second)

	assert.Nil(t, s1Err)
	assert.Nil(t, s2Err)
	assert.Nil(t, s3Err)
	assert.Equal(t, msg, s1Msg)
	assert.Equal(t, msg, s2Msg)
	assert.Equal(t, msg, s3Msg)
	assert.Equal(t, topic, s1Topic)
	assert.Equal(t, topic, s2Topic)
	assert.Equal(t, topic, s3Topic)
}

func TestSubscribe_manyTopics(t *testing.T) {
	topic1 := "topic1"
	topic2 := "topic2"
	msg1 := "msg1"
	msg2 := "msg2"

	s := Subscribe([]string{topic1, topic2})
	PublishString(topic1, msg1)
	PublishString(topic2, msg2)

	s1Topic1, s1Msg1, s1Err1 := s.ReceiveTimeout(time.Second)
	s1Topic2, s1Msg2, s1Err2 := s.ReceiveTimeout(time.Second)

	assert.Equal(t, topic1, s1Topic1)
	assert.Equal(t, msg1, s1Msg1)
	assert.Nil(t, s1Err1)

	assert.Equal(t, topic2, s1Topic2)
	assert.Equal(t, msg2, s1Msg2)
	assert.Nil(t, s1Err2)
}

func TestPublishMarshal(t *testing.T) {
	topic := "topic"
	var msg struct {
		ID      int64
		Msg     string
		private string
	}
	msg.ID = 1
	msg.Msg = "will be sent"
	msg.private = "will not"

	s1 := Subscribe([]string{topic})
	pubErr := Publish(topic, msg)
	s1Topic, s1Msg, s1Err := s1.ReceiveTimeout(time.Second)

	assert.Nil(t, pubErr)
	assert.Nil(t, s1Err)
	assert.Equal(t, `{"ID":1,"Msg":"will be sent"}`, s1Msg)
	assert.Equal(t, topic, s1Topic)
}

func TestSub_Close(t *testing.T) {
	topic := "topic1"

	s1 := Subscribe([]string{topic})
	s1.Close()
	_, _, s1Err := s1.ReceiveTimeout(time.Second)

	assert.Equal(t, ErrCancelled, s1Err)
}
