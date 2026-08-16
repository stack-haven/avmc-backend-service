// Package client 提供 Ark Platform 服务的 gRPC 客户端 SDK，供产品服务（evie 等）复用。
package client

import (
	"context"

	pb "backend-service/api/platform/service/v1"
	"backend-service/pkg/audit"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/utils/convert"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var _ audit.Client = (*AuditClient)(nil)

// AuditClient 通过 gRPC 委托 platform 的 OperationLogService.CreateOperationLog 写入操作审计。
type AuditClient struct {
	conn   *grpc.ClientConn
	client pb.OperationLogServiceClient
}

// NewAuditClient 创建 gRPC 审计委托客户端，连接 platform 服务。
func NewAuditClient(ctx context.Context, endpoint string) (*AuditClient, error) {
	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	return &AuditClient{
		conn:   conn,
		client: pb.NewOperationLogServiceClient(conn),
	}, nil
}

// Append 将 audit.Record 转换为 proto OperationLog 并调用 platform 的 CreateOperationLog RPC。
func (c *AuditClient) Append(ctx context.Context, record *audit.Record) error {
	// 原样转发当前请求的 Authorization，让 platform 独立验证审计操作者身份。
	ctx = authn.ForwardAuthToken(ctx)
	entry := &pb.OperationLog{
		TenantId:       record.TenantID,
		OperatorId:     convert.EmptyToNil(record.OperatorID),
		OperatorName:   convert.EmptyToNil(record.OperatorName),
		Module:         record.Module,
		Action:         record.Action,
		ResourceType:   convert.EmptyToNil(record.ResourceType),
		ResourceId:     convert.EmptyToNil(record.ResourceID),
		Method:         convert.EmptyToNil(record.Method),
		Path:           convert.EmptyToNil(record.Path),
		RequestSummary: convert.EmptyToNil(record.RequestSummary),
		Ip:             convert.EmptyToNil(record.IP),
		UserAgent:      convert.EmptyToNil(record.UserAgent),
		TraceId:        convert.EmptyToNil(record.TraceID),
		Success:        record.Success,
		DurationMs:     convert.ToPointer(record.DurationMs),
		ErrorMessage:   convert.EmptyToNil(record.ErrorMessage),
	}
	_, err := c.client.CreateOperationLog(ctx, &pb.CreateOperationLogRequest{Entry: entry})
	return err
}

// Close 关闭 gRPC 连接。
func (c *AuditClient) Close() error { return c.conn.Close() }
