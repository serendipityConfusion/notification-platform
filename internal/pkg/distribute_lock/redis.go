package distribute_lock

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"sync"
	"time"
)

type RedisDistributeLock struct {
	client *redis.Client
}

func (r *RedisDistributeLock) NewLock(ctx context.Context, key string, opts *LockerOption) DistributeMuter {
	return NewDistributeMutex(ctx, r.client, key, opts)
}

func NewRedisDistributeClient(rdb *redis.Client) Client {
	return &RedisDistributeLock{client: rdb}
}

var (
	// redis.status_reply("OK") 返回string
	luaTryLock = `if redis.call("set", KEYS[1], ARGV[1], "EX", ARGV[2], "NX") then return 0 else return -1 end`
	luaGetDel  = `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call('del', KEYS[1]) else return -1 end`

	ErrLockFailed   = errors.New("err lock false")
	ErrUnLockFailed = errors.New("err unlock false")
)

type DistributeMutex struct {
	ctx     context.Context
	client  *redis.Client
	key     string
	lock    sync.Mutex
	value   string
	options *LockerOption
}

var _ DistributeMuter = (*DistributeMutex)(nil)

func NewLockerOption(expiration time.Duration, retry int, retryDelay time.Duration) *LockerOption {
	return &LockerOption{
		Expiration: expiration,
		RetryCount: retry,
		RetryDelay: retryDelay,
	}
}

func NewDistributeMutex(ctx context.Context, client *redis.Client, key string, opts *LockerOption) *DistributeMutex {
	return &DistributeMutex{ctx: ctx, client: client, key: key, value: uuid.New().String(), options: opts}
}

func (dm *DistributeMutex) tryLock() (bool, error) {
	result, err := dm.client.Eval(dm.ctx, luaTryLock, []string{dm.key}, dm.value, int(dm.options.Expiration.Seconds())).Int()
	if err != nil {
		return false, err
	}
	if result == -1 {
		return false, nil
	}
	return true, nil
}

func (dm *DistributeMutex) Lock() error {
	dm.lock.Lock()
	defer dm.lock.Unlock()
	retryCount := dm.options.RetryCount
	ticker := time.NewTicker(dm.options.RetryDelay)
	defer ticker.Stop()
	for {
		ok, err := dm.tryLock()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if retryCount <= 0 {
			return ErrLockFailed
		}
		retryCount--
		<-ticker.C
	}
}

func (dm *DistributeMutex) Unlock() error {
	result, err := dm.client.Eval(dm.ctx, luaGetDel, []string{dm.key}, dm.value).Int()
	if err != nil {
		return err
	}
	if result == -1 {
		return ErrUnLockFailed
	}
	return nil
}
