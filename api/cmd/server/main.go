package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	db "github.com/can-you-buy-me/db/sqlc"
	"github.com/can-you-buy-me/internal/auth"
	"github.com/can-you-buy-me/internal/database"
	grpcpkg "github.com/can-you-buy-me/internal/grpc"
	"github.com/can-you-buy-me/internal/handlers"
	"github.com/can-you-buy-me/internal/messaging"
	"github.com/can-you-buy-me/internal/middleware"
	"github.com/can-you-buy-me/internal/payment"
	"github.com/can-you-buy-me/internal/security"
	"github.com/can-you-buy-me/internal/webhook"
	auctionpb "github.com/can-you-buy-me/pkg/gen/auction/v1"
	authpb "github.com/can-you-buy-me/pkg/gen/auth/v1"
	healthpb "github.com/can-you-buy-me/pkg/gen/health/v1"
	paymentpb "github.com/can-you-buy-me/pkg/gen/payment/v1"
)

func main() {
	// Cargar variables de entorno desde .env
	_ = godotenv.Load()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("Starting Can You Buy Me API Server...")

	// Verificar configuración de Stripe (puede ser nil si no está configurada)
	var stripeClient *payment.StripeClient
	client, err := payment.NewStripeClient()
	if err != nil {
		slog.Warn("Stripe not configured (STRIPE_API_KEY not set or invalid)", "error", err)
		slog.Warn("Payment features will be disabled")
	} else {
		// Verificar conectividad con Stripe
		account, err := client.VerifyConnection()
		if err != nil {
			slog.Error("Failed to verify Stripe connection", "error", err)
			os.Exit(1)
		}

		stripeClient = client
		mode := stripeClient.GetMode()
		slog.Info("Stripe connection verified",
			"mode", mode,
			"account_id", account.ID,
			"account_email", account.Email,
			"account_name", account.Settings.Dashboard.DisplayName,
		)
	}

	// Conectar a PostgreSQL
	dbConn, err := database.New()
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer dbConn.Close()

	slog.Info("Database connection established",
		"host", getEnvOrDefault("DB_HOST", "localhost"),
		"port", getEnvOrDefault("DB_PORT", "5435"),
		"name", getEnvOrDefault("DB_NAME", "auction_db"),
	)

	// Configurar servidor gRPC
	grpcConfig := &grpcpkg.ServerConfig{
		GRPCPort:   getEnvOrDefault("GRPC_PORT", "50051"),
		HTTPPort:   getEnvOrDefault("HTTP_PORT", "8070"),
		MaxMsgSize: 10 * 1024 * 1024, // 10 MiB
	}

	slog.Info("Server configuration",
		"grpc_port", grpcConfig.GRPCPort,
		"http_port", grpcConfig.HTTPPort,
	)

	// Crear JWT manager (falla en producción si no hay JWT_SECRET)
	jwtManager, err := security.NewJWTManager()
	if err != nil {
		slog.Error("Failed to initialize JWT manager", "error", err)
		os.Exit(1)
	}

	// Crear servidor gRPC con AuthInterceptor cableado
	grpcServer, grpcListener, err := grpcpkg.NewGRPCServer(
		grpcConfig,
		grpc.ChainUnaryInterceptor(
			middleware.AuthInterceptor(jwtManager),
		),
	)
	if err != nil {
		slog.Error("Failed to create gRPC server", "error", err)
		os.Exit(1)
	}

	// Crear queries para base de datos (generado por sqlc)
	queries := db.New(dbConn)

	// Conectar a NATS JetStream. Es el NÚCLEO del producto (bus de pujas en
	// tiempo real + libro de auditoría): si no conecta, no hay negocio → fail-fast
	// (a diferencia de Stripe, que degrada).
	natsURL := getEnvOrDefault("NATS_URL", "nats://localhost:4222")
	natsClient, err := messaging.New(natsURL)
	if err != nil {
		slog.Error("Failed to connect to NATS", "error", err, "url", natsURL)
		os.Exit(1)
	}
	defer natsClient.Close()

	bootstrapCtx, bootstrapCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := natsClient.EnsureStream(bootstrapCtx); err != nil {
		bootstrapCancel()
		slog.Error("Failed to ensure NATS stream", "error", err)
		os.Exit(1)
	}
	if err := natsClient.EnsureKV(bootstrapCtx); err != nil {
		bootstrapCancel()
		slog.Error("Failed to ensure NATS KV bucket", "error", err)
		os.Exit(1)
	}
	bootstrapCancel()
	slog.Info("NATS JetStream ready",
		"url", natsURL,
		"stream", messaging.StreamName,
		"kv", messaging.KVBucket,
	)

	// Arrancar el audit consumer: drena las pujas del stream a Postgres de forma
	// asíncrona (fuera del camino caliente de la puja).
	auditCC, err := natsClient.StartAuditConsumer(context.Background(), queries)
	if err != nil {
		slog.Error("Failed to start bid audit consumer", "error", err)
		os.Exit(1)
	}
	defer auditCC.Stop()
	slog.Info("Bid audit consumer started", "durable", "audit-postgres")

	// Registrar servicios gRPC
	healthHandler := handlers.NewHealthHandler()
	healthpb.RegisterHealthServiceServer(grpcServer, healthHandler)

	// Construir los verificadores de proveedores OAuth (Google hoy; Apple después).
	verifiers := map[string]auth.ProviderVerifier{}
	if googleClientID := getEnvOrDefault("GOOGLE_CLIENT_ID", ""); googleClientID != "" {
		verifiers["google"] = auth.NewGoogleVerifier(googleClientID)
		slog.Info("Google OAuth verifier enabled")
	} else {
		slog.Warn("GOOGLE_CLIENT_ID not set — Google login disabled")
	}

	// Registrar AuthService
	authHandler := handlers.NewAuthHandler(dbConn.Pool, queries, jwtManager, verifiers)
	authpb.RegisterAuthServiceServer(grpcServer, authHandler)
	slog.Info("Auth service registered")

	// Registrar PaymentService (solo si Stripe está configurado)
	if stripeClient != nil {
		paymentHandler := handlers.NewPaymentHandler(queries, stripeClient)
		paymentpb.RegisterPaymentServiceServer(grpcServer, paymentHandler)
		slog.Info("Payment service registered")
	} else {
		slog.Warn("Payment service disabled (Stripe not configured)")
	}

	// Registrar AuctionService (motor de pujas en tiempo real)
	auctionHandler := handlers.NewAuctionHandler(queries, natsClient)
	auctionpb.RegisterAuctionServiceServer(grpcServer, auctionHandler)
	slog.Info("Auction service registered")

	// Registrar reflexión para herramientas como grpcurl
	reflection.Register(grpcServer)

	// Iniciar servidor gRPC en goroutine
	go func() {
		slog.Info("gRPC server listening", "port", grpcConfig.GRPCPort)
		if err := grpcServer.Serve(grpcListener); err != nil {
			slog.Error("gRPC server failed", "error", err)
		}
	}()

	// Crear handler gRPC-Web (sin Envoy)
	grpcWebHandler := grpcpkg.NewGRPCWebHandler(grpcServer)

	// Crear servidor HTTP que sirve gRPC-Web y webhooks
	httpMux := http.NewServeMux()

	// Health check HTTP simple
	httpMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Registrar webhook de Stripe (si está configurado)
	webhookHandler, err := webhook.NewStripeWebhookHandler()
	if err != nil {
		slog.Warn("Stripe webhook disabled (STRIPE_WEBHOOK_SECRET not set)", "error", err)
	} else {
		httpMux.HandleFunc("/webhooks/stripe", webhookHandler.Handle)
		slog.Info("Stripe webhook endpoint registered", "path", "/webhooks/stripe")
	}

	// Ruta para gRPC-Web (catch-all, debe ir al final)
	httpMux.Handle("/", grpcWebHandler)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%s", grpcConfig.HTTPPort),
		Handler:      httpMux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Iniciar servidor HTTP en goroutine
	go func() {
		slog.Info("HTTP/gRPC-Web server listening", "port", grpcConfig.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server failed", "error", err)
		}
	}()

	// Health check de prueba
	time.Sleep(500 * time.Millisecond)
	testHealthCheck(grpcConfig.HTTPPort)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down servers...")

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}

	// Stop gRPC server
	grpcServer.GracefulStop()

	slog.Info("Servers stopped successfully")
}

func testHealthCheck(httpPort string) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%s/health", httpPort))
	if err != nil {
		slog.Warn("Health check test failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		slog.Info("Health check successful")
	} else {
		slog.Warn("Health check returned non-OK status", "status", resp.StatusCode)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
