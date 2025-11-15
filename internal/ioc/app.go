package ioc

import "google.golang.org/grpc"

type App struct {
	GrpcServer *grpc.Server
}
