package client

import (
	"context"
	"net"
	"testing"

	pb "backend-service/api/platform/service/v1"
	"backend-service/pkg/audit"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestAuditClientAppend(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	pb.RegisterOperationLogServiceServer(srv, &stubOpLogServer{})
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	c := &AuditClient{
		conn:   conn,
		client: pb.NewOperationLogServiceClient(conn),
	}

	err = c.Append(context.Background(), &audit.Record{
		TenantID:     100,
		OperatorID:   1,
		OperatorName: "test",
		Module:       "evie.dictionary",
		Action:       "/evie.service.v1.DictionaryService/CreateWord",
		Success:      true,
	})
	if err != nil {
		t.Fatalf("Append error: %v", err)
	}
}

type stubOpLogServer struct {
	pb.UnimplementedOperationLogServiceServer
}

func (s *stubOpLogServer) CreateOperationLog(_ context.Context, _ *pb.CreateOperationLogRequest) (*pb.CreateOperationLogResponse, error) {
	return &pb.CreateOperationLogResponse{}, nil
}

func (s *stubOpLogServer) ListOperationLogs(_ context.Context, _ *pb.ListOperationLogsRequest) (*pb.ListOperationLogsResponse, error) {
	return &pb.ListOperationLogsResponse{}, nil
}

func (s *stubOpLogServer) GetOperationLog(_ context.Context, _ *pb.GetOperationLogRequest) (*pb.OperationLog, error) {
	return &pb.OperationLog{}, nil
}
