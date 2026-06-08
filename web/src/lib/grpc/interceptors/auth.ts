import type { Interceptor } from "@connectrpc/connect";

import { getToken } from "@/lib/auth";

/**
 * Adjunta el JWT como `Authorization: Bearer <token>` a cada request
 * gRPC-Web saliente, si hay sesión activa.
 *
 * Es transparente: las RPCs públicas (p.ej. HealthService) simplemente
 * viajan sin header cuando no hay token. El backend decide qué métodos
 * exigen autenticación vía su interceptor de servidor.
 */
export const authInterceptor: Interceptor = (next) => async (req) => {
  const token = getToken();
  if (token) {
    req.header.set("Authorization", `Bearer ${token}`);
  }
  return next(req);
};
