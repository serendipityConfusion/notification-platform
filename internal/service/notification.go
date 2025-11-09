package service

import (
	"context"
	"github.com/serendipityConfusion/notification-platform/internal/domain"
	"github.com/serendipityConfusion/notification-platform/internal/repository"
)

//go:generate mockgen -source=./notification.go -destination=./mocks/notification.mock.go -package=notificationmocks -typed Service
type Service interface {
	// FindReadyNotifications 准备好调度发送的通知
	FindReadyNotifications(ctx context.Context, offset, limit int) ([]domain.Notification, error)
	// GetByKeys 根据业务ID和业务内唯一标识获取通知列表
	GetByKeys(ctx context.Context, bizID int64, keys ...string) ([]domain.Notification, error)
}

var _ Service = &notificationService{}

type notificationService struct {
	repo repository.NotificationRepository
}

func (n notificationService) FindReadyNotifications(ctx context.Context, offset, limit int) ([]domain.Notification, error) {
	//TODO implement me
	panic("implement me")
}

func (n notificationService) GetByKeys(ctx context.Context, bizID int64, keys ...string) ([]domain.Notification, error) {
	//TODO implement me
	panic("implement me")
}
