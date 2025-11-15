package grpc

import (
	"context"
	notificationpb "github.com/serendipityConfusion/notification-platform/api/gen/v1"
	notificationsvc "github.com/serendipityConfusion/notification-platform/internal/service"
)

type NotificationServer struct {
	notificationpb.UnimplementedNotificationServiceServer
	notificationpb.UnimplementedNotificationQueryServiceServer

	notificationSvc notificationsvc.Service
}

func NewServer(service notificationsvc.Service) *NotificationServer {
	return &NotificationServer{
		notificationSvc: service,
	}
}

func (n NotificationServer) SendNotification(ctx context.Context, request *notificationpb.SendNotificationRequest) (*notificationpb.SendNotificationResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (n NotificationServer) SendNotificationAsync(ctx context.Context, request *notificationpb.SendNotificationAsyncRequest) (*notificationpb.SendNotificationAsyncResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (n NotificationServer) BatchSendNotifications(ctx context.Context, request *notificationpb.BatchSendNotificationsRequest) (*notificationpb.BatchSendNotificationsResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (n NotificationServer) BatchSendNotificationsAsync(ctx context.Context, request *notificationpb.BatchSendNotificationsAsyncRequest) (*notificationpb.BatchSendNotificationsAsyncResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (n NotificationServer) TxPrepare(ctx context.Context, request *notificationpb.TxPrepareRequest) (*notificationpb.TxPrepareResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (n NotificationServer) TxCommit(ctx context.Context, request *notificationpb.TxCommitRequest) (*notificationpb.TxCommitResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (n NotificationServer) TxCancel(ctx context.Context, request *notificationpb.TxCancelRequest) (*notificationpb.TxCancelResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (n NotificationServer) mustEmbedUnimplementedNotificationServiceServer() {
	//TODO implement me
	panic("implement me")
}

var _ notificationpb.NotificationServiceServer = (*NotificationServer)(nil)
var _ notificationpb.NotificationQueryServiceServer = (*NotificationServer)(nil)
