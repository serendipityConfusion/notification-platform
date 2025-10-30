package ioc

import (
	"github.com/google/wire"
	"github.com/serendipityConfusion/notification-platform/internal/ioc"
)

var (
	BaseSet = wire.NewSet(
		ioc.InitDB,
		ioc.InitDistributedLock,
	)
)
