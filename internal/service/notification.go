package service

import (
	"context"
	"fmt"
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

func NewNotificationService(repo repository.NotificationRepository) Service {
	return &notificationService{
		repo: repo,
	}
}

type notificationService struct {
	repo repository.NotificationRepository
}

// FindReadyNotifications 准备好调度发送的通知
func (s *notificationService) FindReadyNotifications(ctx context.Context, offset, limit int) ([]domain.Notification, error) {
	return s.repo.FindReadyNotifications(ctx, offset, limit)
}

// GetByKeys 根据业务ID和业务内唯一标识获取通知列表
func (s *notificationService) GetByKeys(ctx context.Context, bizID int64, keys ...string) ([]domain.Notification, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("%w: 业务内唯一标识列表空", domain.ErrInvalidParameter)
	}

	notifications, err := s.repo.GetByKeys(ctx, bizID, keys...)
	if err != nil {
		return nil, fmt.Errorf("获取通知列表失败: %w", err)
	}
	return notifications, nil
}
