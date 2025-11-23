package redisstore

import (
	"context"
	"strconv"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type Store struct {
	client *redis.Client
}

func New(addr string) *Store {
	c := redis.NewClient(&redis.Options{Addr: addr})
	return &Store{client: c}
}

func NewWithOptions(opts *redis.Options) *Store {
	c := redis.NewClient(opts)
	return &Store{client: c}
}

func (s *Store) ReserveCode(ctx context.Context, code string, ttl time.Duration) (bool, error) {
	key := "msg:" + code
	ok, err := s.client.SetNX(ctx, key, "", ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (s *Store) AttachCipher(ctx context.Context, code string, ciphertext string, ttl time.Duration) (bool, error) {
	key := "msg:" + code
	script := redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if not v then return -1 end
if v ~= '' then return 0 end
redis.call('SET', KEYS[1], ARGV[1], 'EX', ARGV[2])
return 1
`)
	ttlSec := int(ttl / time.Second)
	res, err := script.Run(ctx, s.client, []string{key}, ciphertext, strconv.Itoa(ttlSec)).Int()
	if err != nil {
		return false, err
	}
	if res == 1 {
		return true, nil
	}
	if res == 0 {
		return false, nil
	}
	return false, nil
}

func (s *Store) GetAndDelete(ctx context.Context, code string) (string, bool, error) {
	key := "msg:" + code
	v, err := s.client.GetDel(ctx, key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if v == "" {
		return "", false, nil
	}
	return v, true, nil
}
