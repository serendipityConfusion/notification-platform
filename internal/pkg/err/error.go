package err

import "errors"

// 通用错误定义
var (
	ErrToAsync             = errors.New("服务崩溃已转异步")
	ErrExceedLimit         = errors.New("抢资源超出限制")
	ErrErrorConditionIsMet = errors.New("错误事件出现次数或错误率达到阈值")
)
