// Package grpc provides a gRPC-based audit client that converts
// audit.Record entries to proto OperationLog and forwards them
// to the platform's OperationLogService.
package grpc

import (
	"context"

	pb "backend-service/api/core/service/v1"
	"backend-service/pkg/audit"
	"backend-service/pkg/utils/convert"

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

// Close closes the gRPC connection.
func (c *Client) Close() error { return c.conn.Close() }
