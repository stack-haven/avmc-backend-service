package grpc

import (
	"context"
	"net"
	"testing"

	pb "backend-service/api/core/service/v1"
	"backend-service/pkg/auth/authz"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func dialer(lis *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
	return func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}
}

func TestAuthorizerEnforce(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	pb.RegisterAuthServiceServer(srv, &stubAuthServer{})
	go func() { srv.Serve(lis) }()
	defer srv.Stop()

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(dialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	a := &Authorizer{client: pb.NewAuthServiceClient(conn), conn: conn}

	ok, err := a.Enforce(context.Background(), "user:1", "/evie/v1/dict", "POST", "100")
	if err != nil {
		t.Fatalf("Enforce error: %v", err)
	}
	if !ok {
		t.Fatal("expected authorized")
	}
}

func TestAuthorizerEnforceDenied(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	pb.RegisterAuthServiceServer(srv, &stubAuthServer{deny: true})
	go func() { srv.Serve(lis) }()
	defer srv.Stop()

	conn, _ := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(dialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	defer conn.Close()

	a := &Authorizer{client: pb.NewAuthServiceClient(conn), conn: conn}

	ok, err := a.Enforce(context.Background(), "user:2", "/evie/v1/hotword", "DELETE", "100")
	if err == nil {
		t.Fatal("expected permission denied error")
	}
	if ok {
		t.Fatal("expected not authorized")
	}
}

func TestAuthorizerBatchEnforce(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	pb.RegisterAuthServiceServer(srv, &stubAuthServer{})
	go func() { srv.Serve(lis) }()
	defer srv.Stop()

	conn, _ := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(dialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	defer conn.Close()

	a := &Authorizer{client: pb.NewAuthServiceClient(conn), conn: conn}

	results, err := a.BatchEnforce(context.Background(),
		[]authz.Subject{"user:1", "user:2"},
		[]authz.Object{"/evie/v1/a", "/evie/v1/b"},
		[]authz.Action{"GET", "POST"},
		[]authz.Tenant{"100", "100"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || !results[0] || !results[1] {
		t.Fatalf("batch results: %v", results)
	}
}

// stubAuthServer implements AuthServiceServer for testing.
type stubAuthServer struct {
	pb.UnimplementedAuthServiceServer
	deny bool
}

func (s *stubAuthServer) IsAuthorized(_ context.Context, _ *pb.IsAuthorizedRequest) (*pb.IsAuthorizedResponse, error) {
	if s.deny {
		return nil, grpc.Errorf(7, "permission denied")
	}
	return &pb.IsAuthorizedResponse{}, nil
}

func (s *stubAuthServer) Register(_ context.Context, _ *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	return nil, nil
}
