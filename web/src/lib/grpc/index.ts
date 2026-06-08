/**
 * Punto de entrada del cliente gRPC-Web.
 * Importa siempre desde "@/lib/grpc", nunca desde los archivos generados en gen/.
 */
export * from "./clients";
export { transport } from "./transport";
