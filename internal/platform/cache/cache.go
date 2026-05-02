package cache

import (
	"context"
	"errors"
	"time"
)

var ErrMiss = errors.New("cache miss")

type Cache interface {
	Get(ctx context.Context, key string, dest any) error
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	DeleteByPattern(ctx context.Context, pattern string) error
	Ping(ctx context.Context) error
	Close() error
}

type Noop struct{}

func (Noop) Get(context.Context, string, any) error                { return ErrMiss }
func (Noop) Set(context.Context, string, any, time.Duration) error { return nil }
func (Noop) DeleteByPattern(context.Context, string) error         { return nil }
func (Noop) Ping(context.Context) error                            { return errors.New("cache disabled") }
func (Noop) Close() error                                          { return nil }
