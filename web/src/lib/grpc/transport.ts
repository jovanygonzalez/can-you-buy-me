import { createGrpcWebTransport } from "@connectrpc/connect-web";

import { authInterceptor } from "./interceptors/auth";

/**
 * Endpoint HTTP del backend que sirve gRPC-Web (api/internal/grpc/server.go,
 * StartHTTPServer en :8070). Configurable por entorno con PUBLIC_GRPC_BASE_URL
 * — debe llevar prefijo PUBLIC_ para que Astro lo exponga al cliente.
 */
const baseUrl =
  (import.meta.env.PUBLIC_GRPC_BASE_URL as string | undefined) ??
  "http://localhost:8070";

/**
 * Transport único compartido por todos los clientes gRPC-Web.
 * Aquí se enchufan los interceptores transversales (JWT, y a futuro:
 * retries, logging, telemetría).
 */
export const transport = createGrpcWebTransport({
  baseUrl,
  interceptors: [authInterceptor],
});
