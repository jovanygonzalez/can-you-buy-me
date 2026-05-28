package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nats-io/nats.go/jetstream"

	db "github.com/can-you-buy-me/db/sqlc"
)

const auditConsumerName = "audit-postgres"

// StartAuditConsumer crea (o actualiza) el Pull Consumer durable que drena las
// pujas del stream a PostgreSQL de forma asíncrona — fuera del camino caliente
// de la puja. Devuelve el ConsumeContext para detenerlo en el shutdown.
func (c *Client) StartAuditConsumer(ctx context.Context, queries *db.Queries) (jetstream.ConsumeContext, error) {
	stream, err := c.js.Stream(ctx, StreamName)
	if err != nil {
		return nil, fmt.Errorf("lookup stream %s: %w", StreamName, err)
	}

	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       auditConsumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "auction.*.bids",
		MaxDeliver:    5,
	})
	if err != nil {
		return nil, fmt.Errorf("create audit consumer: %w", err)
	}

	return cons.Consume(func(msg jetstream.Msg) {
		c.handleAuditMsg(queries, msg)
	})
}

// handleAuditMsg persiste una puja en Postgres. La unicidad de stream_seq hace
// el INSERT idempotente, así que la entrega "al menos una vez" de JetStream es
// segura: una reentrega tras un Ack perdido no duplica la puja.
func (c *Client) handleAuditMsg(queries *db.Queries, msg jetstream.Msg) {
	var ev BidEvent
	if err := json.Unmarshal(msg.Data(), &ev); err != nil {
		// Envelope corrupto: no reintentar (Term lo saca de la cola).
		slog.Error("audit: invalid bid envelope, terminating", "error", err)
		_ = msg.Term()
		return
	}

	meta, err := msg.Metadata()
	if err != nil {
		slog.Error("audit: missing JetStream metadata, will retry", "error", err, "bid_id", ev.BidID)
		_ = msg.Nak()
		return
	}

	var amount pgtype.Numeric
	if err := amount.Scan(strconv.FormatFloat(ev.Amount, 'f', 2, 64)); err != nil {
		slog.Error("audit: invalid bid amount, terminating", "error", err, "bid_id", ev.BidID)
		_ = msg.Term()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = queries.CreateBid(ctx, db.CreateBidParams{
		AuctionID: ev.AuctionID,
		UserID:    ev.UserID,
		BidAmount: amount,
		IpAddress: pgtype.Text{}, // null por ahora (no se propaga en el envelope)
		UserAgent: pgtype.Text{}, // null por ahora
		StreamSeq: pgtype.Int8{Int64: int64(meta.Sequence.Stream), Valid: true},
	})
	if err != nil {
		// Reentrega de una puja ya persistida (stream_seq UNIQUE) → tratar como éxito.
		if isUniqueViolation(err) {
			_ = msg.Ack()
			return
		}
		// Fallo transitorio (DB caída, timeout): NO Ack → JetStream reintrega.
		slog.Error("audit: failed to persist bid, will retry", "error", err, "bid_id", ev.BidID)
		_ = msg.Nak()
		return
	}

	slog.Debug("audit: bid persisted", "bid_id", ev.BidID, "stream_seq", meta.Sequence.Stream)
	_ = msg.Ack()
}

// isUniqueViolation detecta el error 23505 (unique_violation) de PostgreSQL.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
