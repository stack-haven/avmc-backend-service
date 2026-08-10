// Package grpc provides a gRPC-based audit client that converts
// audit.Record entries to proto OperationLog and forwards them
// to the platform's OperationLogService.
package grpc

import (
	"context"

	pb "backend-service/api/core/service/v1"
	"backend-service/pkg/audit"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var _ audit.Client = (*Client)(nil)

// Client forwards operation logs to the platform via gRPC.
type Client struct {
	conn   *grpc.ClientConn
	client pb.OperationLogServiceClient
}

// New creates a gRPC audit client connected to the platform.
func New(ctx context.Context, endpoint string) (*Client, error) {
	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	return &Client{
		conn:   conn,
		client: pb.NewOperationLogServiceClient(conn),
	}, nil
}

// Append converts an audit.Record to a proto OperationLog and calls
// the platform's CreateOperationLog RPC.
func (c *Client) Append(ctx context.Context, record *audit.Record) error {
	entry := &pb.OperationLog{
		TenantId:       record.TenantID,
		OperatorId:     uint32Ptr(record.OperatorID),
		OperatorName:   strPtr(record.OperatorName),
		Module:         record.Module,
		Action:         record.Action,
		ResourceType:   strPtr(record.ResourceType),
		ResourceId:     strPtr(record.ResourceID),
		Method:         strPtr(record.Method),
		Path:           strPtr(record.Path),
		RequestSummary: strPtr(record.RequestSummary),
		Ip:             strPtr(record.IP),
		UserAgent:      strPtr(record.UserAgent),
		TraceId:        strPtr(record.TraceID),
		Success:        record.Success,
		DurationMs:     int64Ptr(record.DurationMs),
		ErrorMessage:   strPtr(record.ErrorMessage),
	}
	_, err := c.client.CreateOperationLog(ctx, &pb.CreateOperationLogRequest{Entry: entry})
	return err
}

// Close closes the gRPC connection.
func (c *Client) Close() error { return c.conn.Close() }

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func uint32Ptr(v uint32) *uint32 {
	if v == 0 {
		return nil
	}
	return &v
}

func int64Ptr(v int64) *int64 { return &v }
