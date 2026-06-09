import { createClient } from "@connectrpc/connect";

import { transport } from "./transport";
import { HealthService } from "./gen/v1/health_pb";
import { AuthService } from "./gen/v1/auth_pb";

/**
 * Clientes gRPC-Web tipados, uno por servicio del contrato .proto.
 *
 * Patrón de bajo mantenimiento: cada nuevo servicio = una línea aquí.
 * Comparten el mismo `transport` (mismo baseUrl + interceptor JWT), así que
 * la auth y la configuración de red se definen UNA sola vez en transport.ts.
 *
 *   const res = await healthClient.check({});
 *
 * A medida que el contrato crezca (Fase 2+), descomenta/añade:
 *   import { AuthService } from "./gen/v1/auth_pb";
 *   export const authClient = createClient(AuthService, transport);
 */
export const healthClient = createClient(HealthService, transport);
export const authClient = createClient(AuthService, transport);
