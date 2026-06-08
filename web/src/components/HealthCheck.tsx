import { useEffect, useState } from "react";

import { healthClient } from "@/lib/grpc";
import { HealthCheckResponse_Status } from "@/lib/grpc/gen/v1/health_pb";

type State =
  | { kind: "loading" }
  | { kind: "ok"; pong: string; status: string }
  | { kind: "error"; message: string };

/**
 * Prueba de extremo a extremo de la Fase 1: llama a HealthService.Ping y
 * HealthService.Check vía gRPC-Web (con el JWT adjuntado por el interceptor,
 * si existe) y muestra el resultado.
 */
export default function HealthCheck() {
  const [state, setState] = useState<State>({ kind: "loading" });

  async function run() {
    setState({ kind: "loading" });
    try {
      const [ping, check] = await Promise.all([
        healthClient.ping({}),
        healthClient.check({}),
      ]);
      setState({
        kind: "ok",
        pong: ping.message,
        status: HealthCheckResponse_Status[check.status],
      });
    } catch (err) {
      setState({
        kind: "error",
        message: err instanceof Error ? err.message : String(err),
      });
    }
  }

  useEffect(() => {
    void run();
  }, []);

  return (
    <div className="rounded-lg border p-4 font-mono text-sm">
      {state.kind === "loading" && <p>⏳ Consultando backend…</p>}

      {state.kind === "ok" && (
        <div className="space-y-1">
          <p className="text-green-600">✅ gRPC-Web OK</p>
          <p>Ping → {state.pong}</p>
          <p>Check → {state.status}</p>
        </div>
      )}

      {state.kind === "error" && (
        <div className="space-y-1">
          <p className="text-red-600">❌ Error: {state.message}</p>
          <p className="text-muted-foreground">
            ¿Está corriendo el backend en gRPC-Web? (make run, :8070)
          </p>
        </div>
      )}

      <button
        type="button"
        onClick={() => void run()}
        className="mt-3 rounded border px-3 py-1 hover:bg-muted"
      >
        Reintentar
      </button>
    </div>
  );
}
