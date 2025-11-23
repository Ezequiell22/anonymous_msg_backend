package storage

import (
	"context"
	"time"
)

type Storage interface {
	ReserveCode(ctx context.Context, code string, ttl time.Duration) (bool, error)
	AttachCipher(ctx context.Context, code string, ciphertext string, ttl time.Duration) (bool, error)
	GetAndDelete(ctx context.Context, code string) (string, bool, error)
}
