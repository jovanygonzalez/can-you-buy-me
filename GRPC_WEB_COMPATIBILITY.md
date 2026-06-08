# Compatibilidad gRPC-Web: Astro + Flutter

> ¿Es un problema real consumir gRPC-Web desde Astro (TS) y Flutter Web (Dart)?
> **Respuesta corta: no** — pero no por suerte, sino por dos razones concretas de la arquitectura de **Can You Buy Me**. Este documento explica de dónde nace la duda y por qué se disuelve.

---

## De dónde nace la "incompatibilidad"

El problema de fondo es real y existe: **los navegadores no pueden hablar gRPC "de verdad".** gRPC nativo corre sobre HTTP/2 con *trailers* y control de framing de bajo nivel que las APIs del browser (`fetch`/`XHR`) no exponen. Por eso ni Astro ni Flutter Web pueden abrir un canal gRPC nativo.

**gRPC-Web** es precisamente la solución a eso: un protocolo *distinto*, variante de gRPC, diseñado para funcionar sobre `fetch`/`XHR`. Mete los trailers en el cuerpo de la respuesta como un frame especial. El precio es que necesita un traductor del lado del servidor.

---

## Por qué en este proyecto ya está resuelto

```
                          ┌─────────────────────────────┐
   Astro (TS/Connect-ES) ─┤                             │
                          │  Go server                  │── :50051 gRPC nativo
   Flutter (Dart/grpc) ───┤  improbable-eng/grpc-web    │
                          │  (WrapServer)               │── :8070 gRPC-WEB  ← navegadores
                          └─────────────────────────────┘
```

Dos cosas clave:

1. **El lado servidor ya está hecho.** El backend usa `improbable-eng/grpc-web` (`grpcweb.WrapServer`), que es un traductor *in-process*. No necesita Envoy ni proxy aparte — el server Go ya habla gRPC-Web en el puerto HTTP 8070. Cualquier cliente gRPC-Web estándar se conecta ahí.

2. **Un solo `.proto`, dos clientes generados.** Astro genera stubs TS (Connect-ES), Flutter genera stubs Dart (`grpc` package con `GrpcWebClientChannel`). Ambos implementan el *mismo* protocolo gRPC-Web definido. Interoperan con el handler de improbable — esto se hace en producción todo el tiempo, no es experimental.

---

## La ÚNICA limitación real de gRPC-Web — y por qué no aplica aquí

gRPC-Web tiene una debilidad genuina:

| Tipo de llamada | gRPC-Web en browser |
|---|---|
| Unary (request → response) | ✅ funciona perfecto |
| Server-streaming | ⚠️ funciona pero con quirks de transporte |
| Client-streaming / bidireccional | ❌ **no existe en gRPC-Web** |

Esta es *la* queja clásica de gRPC-Web. **Pero la arquitectura de este proyecto la esquiva:** el stream de pujas en tiempo real **no va por gRPC-Web** — va por **WebSocket → NATS** (CLAUDE.md: *reads/real-time = WebSocket + NATS*). Por gRPC-Web solo viajan **escrituras unary**:

- `Login`, `Register` → unary
- `PlaceBid` → unary (mandas la puja, recibes ack)
- `SetupIntent` de Stripe → unary

Todas son unary. **Nunca se usa streaming sobre gRPC-Web**, porque el tiempo real está delegado a NATS. Es decir: el diseño esquiva por construcción la única debilidad seria de gRPC-Web. Eso no fue casualidad — es la razón por la que el stack separa "writes por gRPC-Web" de "reads por WebSocket".

---

## Lo que sí debe configurarse bien (3 ajustes, no "problemas")

Que no haya incompatibilidad de fondo no significa cero fricción. Hay tres cosas que hay que hacer correctas o no conecta:

1. **En Astro: usar el transporte correcto.** Connect-ES habla *dos* protocolos: el "Connect protocol" y el "gRPC-Web protocol". El handler de improbable **solo entiende gRPC-Web**. Hay que usar `createGrpcWebTransport` — **nunca** `createConnectTransport`. Este es el footgun #1.

2. **CORS en el backend.** El browser hace preflight y necesita que el server exponga los headers de gRPC (`grpc-status`, `grpc-message`, etc.). El handler ya tiene `grpcweb.WithOriginFunc(...)` — solo hay que permitir el origin del frontend (`localhost:4321` en dev, el dominio en prod). Es config, no código nuevo de fondo.

3. **En Flutter: usar el canal web.** En target web, el `grpc` package usa `GrpcWebClientChannel`, no el canal HTTP/2 nativo (que no compila a web). Es elegir la clase correcta.

---

## Opción futura (no para el MVP)

Si algún día se quiere simplificar: cambiar el backend de `improbable-eng/grpc-web` a **`connectrpc/connect-go`** da gRPC nativo + gRPC-Web + protocolo Connect desde *un solo* handler, y el protocolo Connect es debuggeable con `curl` (POST con JSON plano). Pero `improbable` funciona bien hoy y es wire-compatible — no hay razón para tocarlo ahora.

---

## En una línea

No hay incompatibilidad real porque **(1)** el server ya traduce gRPC-Web sin Envoy, y **(2)** el único uso de gRPC-Web es **unary**, así que la limitación de streaming —la gran pega del protocolo— nunca aplica; el tiempo real ya vive en NATS. Lo único que hay que hacer bien son 3 ajustes de configuración, no resolver un problema arquitectónico.

---

*Relacionado:* `docs/FRONTEND_ARCHITECTURE.md` (plan de integración Astro + Flutter), `CLAUDE.md` (stack y reglas).
