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
	if req.Name == "" {
		return nil, merr.NewValidationError(
			"VALIDATION_FAILED",
			"request validation failed",
			merr.FieldViolation{Field: "name", Description: "must not be empty"},
		)
	}
	return &greeterpb.HelloResponse{
		Message: "Hello " + req.Name,
	}, nil
}
