# app/ — Frontend (Flutter Web)

Cliente Flutter Web de **Can You Buy Me**.

> Placeholder. Aquí va el proyecto Flutter. Para inicializarlo:
>
> ```bash
> cd app
> flutter create . --org com.canyoubuyme --platforms web --project-name can_you_buy_me
> ```

## Estructura prevista (ver CLAUDE.md)

```
app/
  lib/
    main.dart        # Entry point
    screens/         # UI (auth, auction, bid history)
    services/        # gRPC-Web client, WebSocket→NATS, Stripe
    models/          # bids, items, users
    widgets/         # componentes reusables
  pubspec.yaml
```

## Contrato gRPC compartido

Los `.proto` viven en `../proto/v1` (raíz del monorepo, compartidos con el backend Go).
Genera los stubs Dart desde ahí:

```bash
protoc --proto_path=../proto/v1 \
  --dart_out=grpc:lib/gen \
  health.proto auth.proto auction.proto payment.proto
```

## Backend

El backend Go vive en [`../api`](../api). Puertos por defecto: gRPC-Web/HTTP en `8070`.
