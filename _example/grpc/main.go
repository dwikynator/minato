package main

import (
	"log"

	"github.com/dwikynator/minato"
	greeterpb "github.com/dwikynator/minato/_example/grpc/grpc/greeter/v1"
	"github.com/dwikynator/minato/_example/grpc/handler"
	"github.com/dwikynator/minato/merr"
	"github.com/dwikynator/minato/middleware"
	"google.golang.org/grpc"
)

func main() {
	server := minato.New(
		minato.WithAddr(":8080"),
		minato.WithGRPCAddr(":9090"),
		minato.WithGRPCReflection(),
		minato.WithGatewayMuxOptions(merr.WithGatewayErrorHandler()),
	)

	// IMPORTANT: RecoveryPlugin MUST be registered first via UsePlugin.
	// Plugins are appended in order, and grpc-go executes interceptors
	// in registration order (first registered = outermost wrapper).
	// Recovery must be outermost to catch panics from ALL inner interceptors.
	server.UsePlugin(
		middleware.RecoveryPlugin(),
		middleware.RequestIDPlugin(),
		middleware.LoggerPlugin(),
	)

	server.Use(middleware.CORS())

	server.RegisterGRPC(func(s grpc.ServiceRegistrar) {
		greeterpb.RegisterGreeterServiceServer(s, handler.NewGreeterHandler())
	})
	server.RegisterGateway(greeterpb.RegisterGreeterServiceHandlerFromEndpoint)
	if err := server.Run(); err != nil {
		log.Fatal(err)
	}
}
