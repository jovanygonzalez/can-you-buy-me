package grpc

import (
	"fmt"
	"net"
	"net/http"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"google.golang.org/grpc"
)

// ServerConfig contiene la configuración para el servidor gRPC y gRPC-Web
type ServerConfig struct {
	GRPCPort  string
	HTTPPort  string
	MaxMsgSize int
}

// DefaultServerConfig retorna una configuración por defecto
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		GRPCPort:   "50051",
		HTTPPort:   "8070",
		MaxMsgSize: 10 * 1024 * 1024, // 10 MiB
	}
}

// NewGRPCServer crea un nuevo servidor gRPC con la configuración dada
// Las opciones adicionales (como interceptores) se pueden pasar vía extraOpts
func NewGRPCServer(cfg *ServerConfig, extraOpts ...grpc.ServerOption) (*grpc.Server, net.Listener, error) {
	// Escuchar en el puerto gRPC
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to listen on gRPC port %s: %w", cfg.GRPCPort, err)
	}

	// Crear opciones base
	baseOpts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(cfg.MaxMsgSize),
		grpc.MaxSendMsgSize(cfg.MaxMsgSize),
	}

	// Combinar opciones base con opciones adicionales
	opts := append(baseOpts, extraOpts...)

	// Crear servidor gRPC con todas las opciones
	grpcServer := grpc.NewServer(opts...)

	return grpcServer, listener, nil
}

// NewGRPCWebHandler crea un handler HTTP que sirve gRPC-Web
// Este handler envuelve el servidor gRPC y lo hace accesible via HTTP/1.1
func NewGRPCWebHandler(grpcServer *grpc.Server) http.Handler {
	return grpcweb.WrapServer(
		grpcServer,
		grpcweb.WithOriginFunc(func(origin string) bool {
			// En MVP, permitir todos los orígenes
			// TODO: Restringir en producción
			return true
		}),
	)
}

// StartHTTPServer inicia el servidor HTTP que sirve gRPC-Web
func StartHTTPServer(cfg *ServerConfig, handler http.Handler) (*http.Server, error) {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.HTTPPort),
		Handler: handler,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	return server, nil
}
