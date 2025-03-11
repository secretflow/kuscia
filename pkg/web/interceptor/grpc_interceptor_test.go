package interceptor

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net"
	"testing"
	"time"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	pberrorcode "github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
	"github.com/stretchr/testify/assert"
)

func TestGrpcInterceptor(t *testing.T) {

	logger := nlog.NewNLog()

	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			GrpcServerLoggingInterceptor(logger),
			GrpcServerTokenInterceptor("valid-token"),
			GrpcServerMasterRoleInterceptor(),
			UnaryRecoverInterceptor(pberrorcode.ErrorCode_INTERNAL),
		),
		grpc.ChainStreamInterceptor(
			GrpcStreamServerLoggingInterceptor(logger),
			GrpcStreamServerTokenInterceptor("valid-token"),
			GrpcStreamServerMasterRoleInterceptor(),
			StreamRecoverInterceptor(pberrorcode.ErrorCode_INTERNAL),
		),
	)
	v1alpha1.RegisterMockServiceServer(s, &MockService{})
	go func() {
		if err := s.Serve(lis); err != nil {
			t.Fatalf("failed to serve: %v", err)
		}
	}()
	defer s.Stop()

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure(), grpc.WithUnaryInterceptor(GrpcClientTokenInterceptor("valid-token")))
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()
	client := v1alpha1.NewMockServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req := &v1alpha1.MockRequest{Name: "World"}
	resp, err := client.UnaryCall(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "Hello, World", resp.Message)

	stream, err := client.StreamCall(ctx, req)
	assert.NoError(t, err)
	respStream, err := stream.Recv()
	assert.NoError(t, err)
	assert.Equal(t, "Hello, World", respStream.Message)

	connInvalid, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure(), grpc.WithUnaryInterceptor(GrpcClientTokenInterceptor("invalid-token")))
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer connInvalid.Close()
	clientInvalid := v1alpha1.NewMockServiceClient(connInvalid)
	_, err = clientInvalid.UnaryCall(ctx, req)
	assert.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	_, err = client.UnaryCall(ctx, &v1alpha1.MockRequest{Name: "Panic"})
	assert.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// Mock gRPC service
type MockService struct {
	v1alpha1.UnimplementedMockServiceServer
}

func (s *MockService) UnaryCall(ctx context.Context, req *v1alpha1.MockRequest) (*v1alpha1.MockResponse, error) {
	return &v1alpha1.MockResponse{Message: "Hello, " + req.Name}, nil
}

func (s *MockService) StreamCall(req *v1alpha1.MockRequest, stream v1alpha1.MockService_StreamCallServer) error {
	stream.Send(&v1alpha1.MockResponse{Message: "Hello, " + req.Name})
	return nil
}
