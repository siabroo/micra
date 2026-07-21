package rediscache

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

// invalidationMessage is the wire format broadcast on the pub/sub channel.
type invalidationMessage struct {
	Keys []string `json:"keys"`
}

// encodeInvalidation serialises the invalidated keys into a channel payload.
func encodeInvalidation(keys []string) ([]byte, error) {
	return json.Marshal(invalidationMessage{Keys: keys})
}

// decodeInvalidation parses a channel payload back into the invalidated keys.
func decodeInvalidation(payload []byte) ([]string, error) {
	var m invalidationMessage
	if err := json.Unmarshal(payload, &m); err != nil {
		return nil, err
	}
	return m.Keys, nil
}

// Publisher broadcasts key-invalidation events on a Redis pub/sub channel so
// other service instances can drop the affected keys from their caches.
type Publisher struct {
	rdb     redis.Cmdable
	channel string
}

// NewPublisher wraps a redis client (typically Client.Redis()) as a Publisher
// broadcasting on the given channel.
func NewPublisher(rdb redis.Cmdable, channel string) *Publisher {
	return &Publisher{rdb: rdb, channel: channel}
}

// Publish broadcasts an invalidation event for the given keys. Publishing zero
// keys still emits a (empty) message.
func (p *Publisher) Publish(ctx context.Context, keys ...string) error {
	payload, err := encodeInvalidation(keys)
	if err != nil {
		return err
	}
	return p.rdb.Publish(ctx, p.channel, payload).Err()
}

// redisSubscriber is the subset of a redis client used by Subscriber. Both
// *redis.Client and redis.UniversalClient satisfy it.
type redisSubscriber interface {
	Subscribe(ctx context.Context, channels ...string) *redis.PubSub
}

// Subscriber receives key-invalidation events from a Redis pub/sub channel and
// dispatches the decoded keys to a handler.
type Subscriber struct {
	rdb     redisSubscriber
	channel string
}

// NewSubscriber wraps a redis client (typically Client.Redis()) as a Subscriber
// listening on the given channel.
func NewSubscriber(rdb redisSubscriber, channel string) *Subscriber {
	return &Subscriber{rdb: rdb, channel: channel}
}

// Subscribe establishes the subscription and consumes messages in a background
// goroutine until ctx is cancelled, calling handler with the decoded keys for
// each event. Malformed messages are skipped. Subscribe returns once the
// subscription is confirmed (or with an error if it cannot be established); the
// underlying subscription is closed on ctx cancellation.
func (s *Subscriber) Subscribe(ctx context.Context, handler func(keys []string)) error {
	pubsub := s.rdb.Subscribe(ctx, s.channel)
	// Confirm the subscription is established before returning.
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return err
	}
	ch := pubsub.Channel()
	go func() {
		defer func() { _ = pubsub.Close() }()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				keys, err := decodeInvalidation([]byte(msg.Payload))
				if err != nil {
					continue
				}
				handler(keys)
			}
		}
	}()
	return nil
}
