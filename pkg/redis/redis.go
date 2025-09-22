package redis

import (
	"context"
	"time"

	"github.com/flaboy/aira-core/pkg/config"

	redis "github.com/redis/go-redis/v9"
)

var (
	RedisClient *redis.Client
)

var Nil = redis.Nil

func InitRedis() error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     config.Config.RedisAddr,
		Password: config.Config.RedisPassword,
		DB:       config.Config.RedisDB,
	})

	ctx, cFun := context.WithTimeout(context.Background(), time.Second)
	defer cFun()

	_, err := RedisClient.Ping(ctx).Result()
	if err != nil {
		return err
	}
	return nil
}

type mutex struct {
	ctx context.Context
	key string
}

func (m *mutex) Lock(ttl time.Duration) error {
	for {
		cmd := RedisClient.SetNX(m.ctx, m.key, 1, ttl)
		if cmd.Err() != nil {
			return cmd.Err()
		}
		if cmd.Val() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (m *mutex) Unlock() error {
	cmd := RedisClient.Del(m.ctx, m.key)
	return cmd.Err()
}

func Mutex(key string) *mutex {
	return &mutex{
		ctx: context.Background(),
		key: key,
	}
}

// Publish sends a message to a Redis channel.
func Publish(channel, message string) error {
	return RedisClient.Publish(context.Background(), channel, message).Err()
}

// Subscribe listens for messages on a Redis channel and returns a channel for receiving messages.
func Subscribe(channel string) (<-chan *redis.Message, error) {
	sub := RedisClient.Subscribe(context.Background(), channel)
	return sub.Channel(), nil
}
