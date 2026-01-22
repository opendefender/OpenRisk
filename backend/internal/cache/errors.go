package cache

import "errors"

var (
ErrCacheMiss = errors.New("cache miss")
ErrConnectionFailed = errors.New("redis connection failed")
)
