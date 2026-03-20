package handler

import (
	"context"

	greeterpb "github.com/dwikynator/minato/_example/grpc/grpc/greeter/v1"
	"github.com/dwikynator/minato/merr"
)

type GreeterHandler struct {
	greeterpb.UnimplementedGreeterServiceServer
}

func NewGreeterHandler() *GreeterHandler {
	return &GreeterHandler{}
}

func (h *GreeterHandler) SayHello(ctx context.Context, req *greeterpb.HelloRequest) (*greeterpb.HelloResponse, error) {
	// Demonstrate error handling — empty name triggers a validation error
	if req.Name == "" {
		return nil, merr.BadRequest(
			"INVALID_NAME",
			"name must not be empty",
			merr.WithDomain("greeter"),
		)
	}

	return &greeterpb.HelloResponse{
		Message: "Hello " + req.Name,
	}, nil
}
