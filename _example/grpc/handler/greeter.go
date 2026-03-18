package handler

import (
	"context"

	greeterpb "github.com/dwikynator/minato/_example/grpc/grpc/greeter/v1"
)

type GreeterHandler struct {
	greeterpb.UnimplementedGreeterServiceServer
}

func NewGreeterHandler() *GreeterHandler {
	return &GreeterHandler{}
}

func (h *GreeterHandler) SayHello(ctx context.Context, req *greeterpb.HelloRequest) (*greeterpb.HelloResponse, error) {
	return &greeterpb.HelloResponse{
		Message: "Hello " + req.Name,
	}, nil
}
