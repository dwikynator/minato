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

	server.UsePlugin(
		middleware.RequestIDPlugin(),
		middleware.LoggerPlugin(),
		middleware.RecoveryPlugin(),
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
