package grpc

import (
	"context"
	"fmt"

	notificationpb "github.com/serendipityConfusion/notification-platform/api/gen/v1"
	"github.com/serendipityConfusion/notification-platform/internal/domain"
	"github.com/serendipityConfusion/notification-platform/internal/pkg/log"
	"github.com/serendipityConfusion/notification-platform/internal/repository"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type NotificationServer struct {
	notificationpb.UnimplementedNotificationServiceServer
	notificationpb.UnimplementedNotificationQueryServiceServer

	repo   repository.NotificationRepository
	logger log.LoggerInterface
}

func NewServer(repo repository.NotificationRepository, logger log.LoggerInterface) *NotificationServer {
	return &NotificationServer{
		repo:   repo,
		logger: logger,
	}
}

// SendNotification 同步单条发送通知
func (s *NotificationServer) SendNotification(ctx context.Context, req *notificationpb.SendNotificationRequest) (*notificationpb.SendNotificationResponse, error) {
	// 验证请求
	if req.GetNotification() == nil {
		return nil, status.Error(codes.InvalidArgument, "notification is required")
	}

	// 转换为领域模型
	notification, err := s.convertToDomainNotification(ctx, req.Notification)
	if err != nil {
		s.logger.Error("convert to domain notification failed", zap.Error(err))
		return s.buildErrorResponse(0, notificationpb.ErrorCode_INVALID_PARAMETER, err.Error()), nil
	}

	// 验证通知
	if err := notification.Validate(); err != nil {
		s.logger.Error("validate notification failed", zap.Error(err))
		return s.buildErrorResponse(0, notificationpb.ErrorCode_INVALID_PARAMETER, err.Error()), nil
	}

	// 设置发送时间
	notification.SetSendTime()
	notification.Status = domain.SendStatusPending

	// 创建通知记录（带回调日志）
	createdNotification, err := s.repo.CreateWithCallbackLog(ctx, notification)
	if err != nil {
		s.logger.Error("create notification failed", zap.Error(err))
		return s.buildErrorResponse(0, notificationpb.ErrorCode_CREATE_NOTIFICATION_FAILED, err.Error()), nil
	}

	// 同步发送：如果是立即发送，则尝试发送
	// TODO: 集成实际的发送逻辑（调用发送服务）
	sendStatus := notificationpb.SendStatus_PENDING
	if notification.IsImmediate() {
		// 这里应该调用实际的发送服务
		// sendErr := s.sendService.Send(ctx, createdNotification)
		// 暂时标记为成功
		sendStatus = notificationpb.SendStatus_SUCCEEDED
		createdNotification.Status = domain.SendStatusSucceeded
		_ = s.repo.MarkSuccess(ctx, createdNotification)
	}

	return &notificationpb.SendNotificationResponse{
		NotificationId: createdNotification.ID,
		Status:         sendStatus,
		ErrorCode:      notificationpb.ErrorCode_ERROR_CODE_UNSPECIFIED,
		ErrorMessage:   "",
	}, nil
}

// SendNotificationAsync 异步单条发送通知
func (s *NotificationServer) SendNotificationAsync(ctx context.Context, req *notificationpb.SendNotificationAsyncRequest) (*notificationpb.SendNotificationAsyncResponse, error) {
	// 验证请求
	if req.GetNotification() == nil {
		return nil, status.Error(codes.InvalidArgument, "notification is required")
	}

	// 转换为领域模型
	notification, err := s.convertToDomainNotification(ctx, req.Notification)
	if err != nil {
		s.logger.Error("convert to domain notification failed", zap.Error(err))
		return &notificationpb.SendNotificationAsyncResponse{
			NotificationId: 0,
			ErrorCode:      notificationpb.ErrorCode_INVALID_PARAMETER,
			ErrorMessage:   err.Error(),
		}, nil
	}

	// 验证通知
	if err := notification.Validate(); err != nil {
		s.logger.Error("validate notification failed", zap.Error(err))
		return &notificationpb.SendNotificationAsyncResponse{
			NotificationId: 0,
			ErrorCode:      notificationpb.ErrorCode_INVALID_PARAMETER,
			ErrorMessage:   err.Error(),
		}, nil
	}

	// 异步发送：如果是立即发送策略，替换为默认截止时间策略
	notification.ReplaceAsyncImmediate()
	notification.SetSendTime()
	notification.Status = domain.SendStatusPending

	// 创建通知记录（不带回调日志，异步发送由调度器处理）
	createdNotification, err := s.repo.Create(ctx, notification)
	if err != nil {
		s.logger.Error("create notification failed", zap.Error(err))
		return &notificationpb.SendNotificationAsyncResponse{
			NotificationId: 0,
			ErrorCode:      notificationpb.ErrorCode_CREATE_NOTIFICATION_FAILED,
			ErrorMessage:   err.Error(),
		}, nil
	}

	s.logger.Info("notification created for async send",
		zap.Uint64("notification_id", createdNotification.ID),
		zap.String("key", createdNotification.Key))

	return &notificationpb.SendNotificationAsyncResponse{
		NotificationId: createdNotification.ID,
		ErrorCode:      notificationpb.ErrorCode_ERROR_CODE_UNSPECIFIED,
		ErrorMessage:   "",
	}, nil
}

// BatchSendNotifications 同步批量发送通知
func (s *NotificationServer) BatchSendNotifications(ctx context.Context, req *notificationpb.BatchSendNotificationsRequest) (*notificationpb.BatchSendNotificationsResponse, error) {
	if len(req.GetNotifications()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "notifications cannot be empty")
	}

	var results []*notificationpb.SendNotificationResponse
	successCount := int32(0)

	// 批量转换和验证
	notifications := make([]domain.Notification, 0, len(req.Notifications))
	for i, pbNotification := range req.Notifications {
		notification, err := s.convertToDomainNotification(ctx, pbNotification)
		if err != nil {
			s.logger.Error("convert notification failed",
				zap.Int("index", i),
				zap.Error(err))
			results = append(results, s.buildErrorResponse(0, notificationpb.ErrorCode_INVALID_PARAMETER, err.Error()))
			continue
		}

		if err := notification.Validate(); err != nil {
			s.logger.Error("validate notification failed",
				zap.Int("index", i),
				zap.Error(err))
			results = append(results, s.buildErrorResponse(0, notificationpb.ErrorCode_INVALID_PARAMETER, err.Error()))
			continue
		}

		notification.SetSendTime()
		notification.Status = domain.SendStatusPending
		notifications = append(notifications, notification)
	}

	if len(notifications) == 0 {
		return &notificationpb.BatchSendNotificationsResponse{
			Results:      results,
			TotalCount:   int32(len(req.Notifications)),
			SuccessCount: 0,
		}, nil
	}

	// 批量创建
	createdNotifications, err := s.repo.BatchCreateWithCallbackLog(ctx, notifications)
	if err != nil {
		s.logger.Error("batch create notifications failed", zap.Error(err))
		// 所有通知都失败
		for range notifications {
			results = append(results, s.buildErrorResponse(0, notificationpb.ErrorCode_CREATE_NOTIFICATION_FAILED, err.Error()))
		}
		return &notificationpb.BatchSendNotificationsResponse{
			Results:      results,
			TotalCount:   int32(len(req.Notifications)),
			SuccessCount: 0,
		}, nil
	}

	// 构建响应
	succeededNotifications := make([]domain.Notification, 0)
	for _, notification := range createdNotifications {
		sendStatus := notificationpb.SendStatus_PENDING

		// 同步发送：如果是立即发送，则尝试发送
		if notification.IsImmediate() {
			// TODO: 集成实际的发送逻辑
			sendStatus = notificationpb.SendStatus_SUCCEEDED
			notification.Status = domain.SendStatusSucceeded
			succeededNotifications = append(succeededNotifications, notification)
			successCount++
		} else {
			successCount++
		}

		results = append(results, &notificationpb.SendNotificationResponse{
			NotificationId: notification.ID,
			Status:         sendStatus,
			ErrorCode:      notificationpb.ErrorCode_ERROR_CODE_UNSPECIFIED,
			ErrorMessage:   "",
		})
	}

	// 批量更新发送成功的通知状态
	if len(succeededNotifications) > 0 {
		_ = s.repo.BatchUpdateStatusSucceededOrFailed(ctx, succeededNotifications, nil)
	}

	return &notificationpb.BatchSendNotificationsResponse{
		Results:      results,
		TotalCount:   int32(len(req.Notifications)),
		SuccessCount: successCount,
	}, nil
}

// BatchSendNotificationsAsync 异步批量发送通知
func (s *NotificationServer) BatchSendNotificationsAsync(ctx context.Context, req *notificationpb.BatchSendNotificationsAsyncRequest) (*notificationpb.BatchSendNotificationsAsyncResponse, error) {
	if len(req.GetNotifications()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "notifications cannot be empty")
	}

	// 批量转换和验证
	notifications := make([]domain.Notification, 0, len(req.Notifications))
	for i, pbNotification := range req.Notifications {
		notification, err := s.convertToDomainNotification(ctx, pbNotification)
		if err != nil {
			s.logger.Error("convert notification failed",
				zap.Int("index", i),
				zap.Error(err))
			continue
		}

		if err := notification.Validate(); err != nil {
			s.logger.Error("validate notification failed",
				zap.Int("index", i),
				zap.Error(err))
			continue
		}

		notification.ReplaceAsyncImmediate()
		notification.SetSendTime()
		notification.Status = domain.SendStatusPending
		notifications = append(notifications, notification)
	}

	if len(notifications) == 0 {
		return &notificationpb.BatchSendNotificationsAsyncResponse{
			NotificationIds: []uint64{},
		}, nil
	}

	// 批量创建（异步发送不需要回调日志）
	createdNotifications, err := s.repo.BatchCreate(ctx, notifications)
	if err != nil {
		s.logger.Error("batch create notifications failed", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to create notifications")
	}

	// 收集通知ID
	notificationIDs := make([]uint64, 0, len(createdNotifications))
	for _, notification := range createdNotifications {
		notificationIDs = append(notificationIDs, notification.ID)
	}

	s.logger.Info("batch notifications created for async send",
		zap.Int("count", len(notificationIDs)))

	return &notificationpb.BatchSendNotificationsAsyncResponse{
		NotificationIds: notificationIDs,
	}, nil
}

// TxPrepare 准备事务消息
func (s *NotificationServer) TxPrepare(ctx context.Context, req *notificationpb.TxPrepareRequest) (*notificationpb.TxPrepareResponse, error) {
	if req.GetNotification() == nil {
		return nil, status.Error(codes.InvalidArgument, "notification is required")
	}

	// 转换为领域模型
	notification, err := s.convertToDomainNotification(ctx, req.Notification)
	if err != nil {
		s.logger.Error("convert to domain notification failed", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// 验证通知
	if err := notification.Validate(); err != nil {
		s.logger.Error("validate notification failed", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// 设置事务状态为准备中
	notification.Status = domain.SendStatusPrepare
	notification.SetSendTime()

	// 创建通知记录
	createdNotification, err := s.repo.Create(ctx, notification)
	if err != nil {
		s.logger.Error("create tx notification failed", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to prepare transaction")
	}

	s.logger.Info("transaction notification prepared",
		zap.Uint64("notification_id", createdNotification.ID),
		zap.String("key", createdNotification.Key))

	return &notificationpb.TxPrepareResponse{}, nil
}

// TxCommit 提交事务消息
func (s *NotificationServer) TxCommit(ctx context.Context, req *notificationpb.TxCommitRequest) (*notificationpb.TxCommitResponse, error) {
	if req.GetKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "key is required")
	}

	// TODO: 从上下文或请求中获取 bizID
	// 这里需要扩展 proto 定义或使用其他方式传递 bizID
	// 暂时使用一个默认值或从 metadata 获取
	bizID := s.getBizIDFromContext(ctx)
	if bizID == 0 {
		return nil, status.Error(codes.InvalidArgument, "bizID is required")
	}

	// 查询通知
	notification, err := s.repo.GetByKey(ctx, bizID, req.Key)
	if err != nil {
		s.logger.Error("get notification by key failed",
			zap.String("key", req.Key),
			zap.Error(err))
		return nil, status.Error(codes.NotFound, "notification not found")
	}

	// 检查状态
	if notification.Status != domain.SendStatusPrepare {
		s.logger.Warn("notification status is not PREPARE",
			zap.Uint64("notification_id", notification.ID),
			zap.String("status", string(notification.Status)))
		return nil, status.Error(codes.FailedPrecondition, "notification is not in PREPARE status")
	}

	// 更新状态为待发送
	notification.Status = domain.SendStatusPending
	if err := s.repo.UpdateStatus(ctx, notification); err != nil {
		s.logger.Error("update notification status failed",
			zap.Uint64("notification_id", notification.ID),
			zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to commit transaction")
	}

	s.logger.Info("transaction notification committed",
		zap.Uint64("notification_id", notification.ID),
		zap.String("key", notification.Key))

	return &notificationpb.TxCommitResponse{}, nil
}

// TxCancel 取消事务消息
func (s *NotificationServer) TxCancel(ctx context.Context, req *notificationpb.TxCancelRequest) (*notificationpb.TxCancelResponse, error) {
	if req.GetKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "key is required")
	}

	// TODO: 从上下文或请求中获取 bizID
	bizID := s.getBizIDFromContext(ctx)
	if bizID == 0 {
		return nil, status.Error(codes.InvalidArgument, "bizID is required")
	}

	// 查询通知
	notification, err := s.repo.GetByKey(ctx, bizID, req.Key)
	if err != nil {
		s.logger.Error("get notification by key failed",
			zap.String("key", req.Key),
			zap.Error(err))
		return nil, status.Error(codes.NotFound, "notification not found")
	}

	// 检查状态
	if notification.Status != domain.SendStatusPrepare {
		s.logger.Warn("notification status is not PREPARE",
			zap.Uint64("notification_id", notification.ID),
			zap.String("status", string(notification.Status)))
		return nil, status.Error(codes.FailedPrecondition, "notification is not in PREPARE status")
	}

	// 更新状态为已取消
	notification.Status = domain.SendStatusCanceled
	if err := s.repo.UpdateStatus(ctx, notification); err != nil {
		s.logger.Error("update notification status failed",
			zap.Uint64("notification_id", notification.ID),
			zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to cancel transaction")
	}

	s.logger.Info("transaction notification canceled",
		zap.Uint64("notification_id", notification.ID),
		zap.String("key", notification.Key))

	return &notificationpb.TxCancelResponse{}, nil
}

// QueryNotification 查询单条通知
func (s *NotificationServer) QueryNotification(ctx context.Context, req *notificationpb.QueryNotificationRequest) (*notificationpb.QueryNotificationResponse, error) {
	if req.GetKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "key is required")
	}

	bizID := s.getBizIDFromContext(ctx)
	if bizID == 0 {
		return nil, status.Error(codes.InvalidArgument, "bizID is required")
	}

	notification, err := s.repo.GetByKey(ctx, bizID, req.Key)
	if err != nil {
		s.logger.Error("get notification by key failed",
			zap.String("key", req.Key),
			zap.Error(err))
		return nil, status.Error(codes.NotFound, "notification not found")
	}

	return &notificationpb.QueryNotificationResponse{
		Result: s.convertToProtoResponse(notification),
	}, nil
}

// BatchQueryNotifications 批量查询通知
func (s *NotificationServer) BatchQueryNotifications(ctx context.Context, req *notificationpb.BatchQueryNotificationsRequest) (*notificationpb.BatchQueryNotificationsResponse, error) {
	if len(req.GetKeys()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "keys cannot be empty")
	}

	bizID := s.getBizIDFromContext(ctx)
	if bizID == 0 {
		return nil, status.Error(codes.InvalidArgument, "bizID is required")
	}

	notifications, err := s.repo.GetByKeys(ctx, bizID, req.Keys...)
	if err != nil {
		s.logger.Error("get notifications by keys failed",
			zap.Strings("keys", req.Keys),
			zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to query notifications")
	}

	results := make([]*notificationpb.SendNotificationResponse, 0, len(notifications))
	for _, notification := range notifications {
		results = append(results, s.convertToProtoResponse(notification))
	}

	return &notificationpb.BatchQueryNotificationsResponse{
		Results: results,
	}, nil
}

// Helper methods

// convertToDomainNotification 将 proto 通知转换为领域模型
func (s *NotificationServer) convertToDomainNotification(ctx context.Context, pbNotification *notificationpb.Notification) (domain.Notification, error) {
	notification, err := domain.NewNotificationFromAPI(pbNotification)
	if err != nil {
		return domain.Notification{}, err
	}

	// 从上下文获取 bizID
	notification.BizID = s.getBizIDFromContext(ctx)
	if notification.BizID == 0 {
		return domain.Notification{}, fmt.Errorf("bizID is required")
	}

	return notification, nil
}

// convertToProtoResponse 将领域模型转换为 proto 响应
func (s *NotificationServer) convertToProtoResponse(notification domain.Notification) *notificationpb.SendNotificationResponse {
	return &notificationpb.SendNotificationResponse{
		NotificationId: notification.ID,
		Status:         s.convertStatus(notification.Status),
		ErrorCode:      notificationpb.ErrorCode_ERROR_CODE_UNSPECIFIED,
		ErrorMessage:   "",
	}
}

// convertStatus 转换发送状态
func (s *NotificationServer) convertStatus(status domain.SendStatus) notificationpb.SendStatus {
	switch status {
	case domain.SendStatusPrepare:
		return notificationpb.SendStatus_PREPARE
	case domain.SendStatusCanceled:
		return notificationpb.SendStatus_CANCELED
	case domain.SendStatusPending:
		return notificationpb.SendStatus_PENDING
	case domain.SendStatusSucceeded:
		return notificationpb.SendStatus_SUCCEEDED
	case domain.SendStatusFailed:
		return notificationpb.SendStatus_FAILED
	default:
		return notificationpb.SendStatus_SEND_STATUS_UNSPECIFIED
	}
}

// buildErrorResponse 构建错误响应
func (s *NotificationServer) buildErrorResponse(id uint64, errorCode notificationpb.ErrorCode, message string) *notificationpb.SendNotificationResponse {
	return &notificationpb.SendNotificationResponse{
		NotificationId: id,
		Status:         notificationpb.SendStatus_FAILED,
		ErrorCode:      errorCode,
		ErrorMessage:   message,
	}
}

// getBizIDFromContext 从上下文中获取 bizID
// TODO: 实现从 metadata 或其他方式获取 bizID 的逻辑
func (s *NotificationServer) getBizIDFromContext(ctx context.Context) int64 {
	// 这里应该从 gRPC metadata 或其他认证信息中获取
	// 暂时返回一个默认值用于演示
	// 实际使用时需要实现真实的逻辑，比如：
	// md, ok := metadata.FromIncomingContext(ctx)
	// if !ok {
	//     return 0
	// }
	// bizIDStr := md.Get("biz-id")
	// return parseBizID(bizIDStr)
	return 1 // 临时返回默认值
}

// 确保实现了接口
var _ notificationpb.NotificationServiceServer = (*NotificationServer)(nil)
var _ notificationpb.NotificationQueryServiceServer = (*NotificationServer)(nil)
