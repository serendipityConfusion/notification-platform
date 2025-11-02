package ioc

import (
	"time"

	"github.com/sony/sonyflake"
)

/*
MachineID 这个函数用于生成机器ID，确保在分布式环境中每个实例有唯一的标识符，从而避免ID冲突。
方案：redis自增，etcd分布式锁等方式均可实现机器ID的唯一分配，环境变量
	39 bits for time in units of 10 msec
	 8 bits for a sequence number
	16 bits for a machine id
*/

// InitIDGenerator ID生成器初始化
func InitIDGenerator() *sonyflake.Sonyflake {
	// 使用固定设置的ID生成器
	return sonyflake.NewSonyflake(sonyflake.Settings{
		StartTime: time.Now(),
		MachineID: func() (uint16, error) {
			return 1, nil
		},
	})
}
