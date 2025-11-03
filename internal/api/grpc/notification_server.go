package grpc

import notificationpb "github.com/serendipityConfusion/notification-platform/api/gen/v1"

type NotificationServer struct {
	notificationSvc
}

var _ notificationpb.NotificationServiceServer = (*NotificationServer)(nil)
var _ notificationpb.NotificationQueryServiceServer = (*NotificationServer)(nil)
