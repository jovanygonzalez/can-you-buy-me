package handlers

import (
	"context"

	pb "github.com/can-you-buy-me/pkg/gen/health/v1"
)

// HealthHandler maneja los RPCs de salud del servidor
type HealthHandler struct {
	pb.UnimplementedHealthServiceServer
}

// NewHealthHandler crea un nuevo HealthHandler
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Ping responde con un pong simple (para verificar que el servidor está vivo)
func (h *HealthHandler) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{
		Message: "pong",
	}, nil
}

// Check verifica la salud general del servidor
func (h *HealthHandler) Check(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{
		Status: pb.HealthCheckResponse_STATUS_SERVING,
	}, nil
}
