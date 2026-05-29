// Package messaging encapsula la integración con NATS JetStream:
// el bus de tiempo real de pujas y el libro de auditoría inmutable.
package messaging

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	// StreamName es el stream que captura todas las pujas y eventos de control.
	StreamName = "AUCTION_BIDS"
	// KVBucket guarda el precio máximo vigente por subasta (autoridad de validación).
	KVBucket = "auction_state"
)

// BidEvent es el envelope que se publica en auction.<id>.bids.
// El frontend lo recibe por WebSocket y el audit consumer lo persiste en Postgres.
type BidEvent struct {
	BidID     string  `json:"bid_id"`     // UUID — idempotencia (Nats-Msg-Id)
	AuctionID int32   `json:"auction_id"`
	UserID    int32   `json:"user_id"`
	Amount    float64 `json:"amount"`
	CreatedAt int64   `json:"created_at"` // unix millis (referencia; el orden real lo da la secuencia del stream)
}

// HighestState es el valor guardado en el KV para el precio máximo de una subasta.
type HighestState struct {
	Amount float64 `json:"amount"`
	UserID int32   `json:"user_id"`
	BidID  string  `json:"bid_id"`
}

// Client envuelve la conexión NATS, el contexto JetStream y el bucket KV.
type Client struct {
	nc *nats.Conn
	js jetstream.JetStream
	kv jetstream.KeyValue
}

// New conecta a NATS y crea el contexto JetStream.
func New(natsURL string) (*Client, error) {
	nc, err := nats.Connect(natsURL,
		nats.Name("auction-api"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("create JetStream context: %w", err)
	}

	return &Client{nc: nc, js: js}, nil
}

// EnsureStream crea (o actualiza) el stream de pujas. Idempotente.
func (c *Client) EnsureStream(ctx context.Context) error {
	_, err := c.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     StreamName,
		Subjects: []string{"auction.*.bids", "auction.*.control"},
		// Disco: ninguna puja se pierde si el contenedor se reinicia.
		Storage: jetstream.FileStorage,
		// Retiene el historial (auditoría); no borra al leer.
		Retention: jetstream.LimitsPolicy,
		// Si el disco se llena, descarta lo más viejo en vez de rechazar pujas nuevas.
		Discard: jetstream.DiscardOld,
		// Tras 48h la subasta ya se consolidó en Postgres; mantiene el disco barato.
		MaxAge:   48 * time.Hour,
		MaxBytes: 1 << 30, // 1 GiB
		// Ventana de deduplicación por Nats-Msg-Id (reintentos no duplican pujas).
		Duplicates: 2 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("ensure stream %s: %w", StreamName, err)
	}
	return nil
}

// EnsureKV crea (o abre) el bucket KV del precio máximo. Idempotente.
//
// Storage Memory es válido aquí: si el servidor colapsa, el precio real siempre
// puede reconstruirse desde el stream en disco (la fuente de verdad).
// Nota: el TTL de KV es por-bucket (no por-key), así que el precio se limpia al
// cerrar la subasta (borrando la key), no por expiración individual.
func (c *Client) EnsureKV(ctx context.Context) error {
	kv, err := c.js.KeyValue(ctx, KVBucket)
	if errors.Is(err, jetstream.ErrBucketNotFound) {
		kv, err = c.js.CreateKeyValue(ctx, jetstream.KeyValueConfig{
			Bucket:  KVBucket,
			Storage: jetstream.MemoryStorage,
			History: 1,
		})
	}
	if err != nil {
		return fmt.Errorf("ensure KV bucket %s: %w", KVBucket, err)
	}
	c.kv = kv
	return nil
}

// KV expone el bucket del precio máximo (lo usa el handler para el CAS).
func (c *Client) KV() jetstream.KeyValue { return c.kv }

// JS expone el contexto JetStream (lo usa el audit consumer).
func (c *Client) JS() jetstream.JetStream { return c.js }

// PublishBid publica una puja validada en auction.<id>.bids.
// msgID se usa como Nats-Msg-Id (idempotencia). Devuelve la secuencia del stream.
func (c *Client) PublishBid(ctx context.Context, auctionID int32, data []byte, msgID string) (uint64, error) {
	subject := fmt.Sprintf("auction.%d.bids", auctionID)
	ack, err := c.js.Publish(ctx, subject, data, jetstream.WithMsgID(msgID))
	if err != nil {
		return 0, fmt.Errorf("publish to %s: %w", subject, err)
	}
	return ack.Sequence, nil
}

// Close cierra la conexión a NATS.
func (c *Client) Close() {
	if c.nc != nil {
		c.nc.Close()
	}
}
