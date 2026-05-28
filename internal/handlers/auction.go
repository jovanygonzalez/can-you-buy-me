package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	db "github.com/can-you-buy-me/db/sqlc"
	"github.com/can-you-buy-me/internal/messaging"
	"github.com/can-you-buy-me/internal/middleware"
	pb "github.com/can-you-buy-me/pkg/gen/auction/v1"
)

// maxCASAttempts limita los reintentos del compare-and-swap sobre el KV
// cuando varias pujas concurrentes chocan en la misma revisión.
const maxCASAttempts = 5

// AuctionHandler implementa el servicio gRPC de subastas.
type AuctionHandler struct {
	pb.UnimplementedAuctionServiceServer
	queries *db.Queries
	nats    *messaging.Client
}

// NewAuctionHandler crea un nuevo AuctionHandler.
func NewAuctionHandler(queries *db.Queries, natsClient *messaging.Client) *AuctionHandler {
	return &AuctionHandler{
		queries: queries,
		nats:    natsClient,
	}
}

// PlaceBid valida una puja y, si gana, la publica en NATS.
//
// La autoridad del precio máximo vive en el KV de NATS: un compare-and-swap por
// revisión garantiza que, aún con varias instancias de Go, solo una puja "gana"
// cada nivel de precio. Tras ganar el CAS, se publica en auction.<id>.bids para
// el fan-out en tiempo real y la persistencia asíncrona en Postgres.
func (h *AuctionHandler) PlaceBid(ctx context.Context, req *pb.PlaceBidRequest) (*pb.PlaceBidResponse, error) {
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user_id not found in context")
	}

	if req.AuctionId <= 0 || req.BidAmount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "auction_id and bid_amount must be positive")
	}

	// TODO(fase 2): gate de pago. Cuando el webhook de Stripe setee
	// has_active_payment_method=true en la BD, rechazar aquí a quien no tenga
	// tarjeta válida (hoy el flag siempre es false, así que el check bloquearía
	// todas las pujas durante el desarrollo del motor):
	//
	//   user, err := h.queries.GetUserByID(ctx, userID)
	//   if err != nil || !user.HasActivePaymentMethod.Bool {
	//       return nil, status.Error(codes.FailedPrecondition, "no active payment method")
	//   }

	// La subasta debe existir y estar abierta a pujas.
	if _, err := h.queries.GetAuctionForBidding(ctx, req.AuctionId); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "auction is not open for bidding")
	}

	key := fmt.Sprintf("auction.%d.highest", req.AuctionId)
	kv := h.nats.KV()
	bidID := newBidID()

	newState, _ := json.Marshal(messaging.HighestState{
		Amount: req.BidAmount,
		UserID: userID,
		BidID:  bidID,
	})

	for attempt := 0; attempt < maxCASAttempts; attempt++ {
		entry, err := kv.Get(ctx, key)

		switch {
		case errors.Is(err, jetstream.ErrKeyNotFound):
			// Primera puja de la subasta.
			if _, cerr := kv.Create(ctx, key, newState); cerr != nil {
				if errors.Is(cerr, jetstream.ErrKeyExists) {
					continue // otra instancia creó primero → reintentar
				}
				return nil, status.Errorf(codes.Internal, "kv create: %v", cerr)
			}

		case err != nil:
			return nil, status.Errorf(codes.Internal, "kv get: %v", err)

		default:
			var cur messaging.HighestState
			if jerr := json.Unmarshal(entry.Value(), &cur); jerr != nil {
				return nil, status.Errorf(codes.Internal, "kv decode: %v", jerr)
			}
			if req.BidAmount <= cur.Amount {
				return &pb.PlaceBidResponse{
					Success: false,
					Message: fmt.Sprintf("bid must be greater than current highest (%.2f)", cur.Amount),
				}, nil
			}
			if _, uerr := kv.Update(ctx, key, newState, entry.Revision()); uerr != nil {
				continue // conflicto de revisión: otra puja ganó → reintentar
			}
		}

		// Ganó el CAS → publicar (fan-out en vivo + auditoría async).
		ev, _ := json.Marshal(messaging.BidEvent{
			BidID:     bidID,
			AuctionID: req.AuctionId,
			UserID:    userID,
			Amount:    req.BidAmount,
			CreatedAt: time.Now().UnixMilli(),
		})
		seq, perr := h.nats.PublishBid(ctx, req.AuctionId, ev, bidID)
		if perr != nil {
			return nil, status.Errorf(codes.Internal, "publish bid: %v", perr)
		}

		return &pb.PlaceBidResponse{
			Success: true,
			Message: "bid accepted",
			BidId:   int64(seq),
		}, nil
	}

	return nil, status.Error(codes.Aborted, "too much contention placing bid, please retry")
}

// GetAuction devuelve el estado de una subasta (snapshot para quien entra tarde).
// El catálogo viene de Postgres y el precio máximo en vivo se superpone desde el KV.
func (h *AuctionHandler) GetAuction(ctx context.Context, req *pb.GetAuctionRequest) (*pb.Auction, error) {
	a, err := h.queries.GetAuctionByID(ctx, req.AuctionId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "auction not found")
	}

	out := auctionToProto(a)

	// El KV es la autoridad en vivo del precio: si hay valor, es más fresco que la BD.
	if entry, kerr := h.nats.KV().Get(ctx, fmt.Sprintf("auction.%d.highest", req.AuctionId)); kerr == nil {
		var st messaging.HighestState
		if json.Unmarshal(entry.Value(), &st) == nil {
			out.CurrentHighestBid = st.Amount
			out.HighestBidderId = st.UserID
		}
	}

	return out, nil
}

// ListAuctions devuelve el catálogo (filtrable por status).
func (h *AuctionHandler) ListAuctions(ctx context.Context, req *pb.ListAuctionsRequest) (*pb.ListAuctionsResponse, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	var rows []db.Auction
	var err error
	if req.Status != "" {
		rows, err = h.queries.ListAuctionsByStatus(ctx, db.ListAuctionsByStatusParams{
			Status: pgtype.Text{String: req.Status, Valid: true},
			Limit:  limit,
			Offset: offset,
		})
	} else {
		rows, err = h.queries.ListActiveAuctions(ctx, db.ListActiveAuctionsParams{
			Limit:  limit,
			Offset: offset,
		})
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list auctions: %v", err)
	}

	out := make([]*pb.Auction, 0, len(rows))
	for _, a := range rows {
		out = append(out, auctionToProto(a))
	}
	return &pb.ListAuctionsResponse{Auctions: out, Total: int32(len(out))}, nil
}

// auctionToProto mapea una fila de la BD al mensaje proto.
func auctionToProto(a db.Auction) *pb.Auction {
	return &pb.Auction{
		Id:                a.ID,
		Title:             a.Title,
		Subtitle:          textVal(a.Subtitle),
		Description:       textVal(a.Description),
		Category:          textVal(a.Category),
		BasePrice:         numVal(a.BasePrice),
		CurrentHighestBid: numVal(a.CurrentHighestBid),
		ImageUrls:         splitURLs(textVal(a.ImageUrls)),
		ThumbnailUrl:      textVal(a.ThumbnailUrl),
		ScheduledAtUnix:   tsUnix(a.ScheduledAt),
		StartedAtUnix:     tsUnix(a.StartedAt),
		EndedAtUnix:       tsUnix(a.EndedAt),
		Status:            textVal(a.Status),
		WinnerId:          int4Val(a.WinnerID),
		HighestBidderId:   int4Val(a.HighestBidderID),
		Sku:               textVal(a.Sku),
		Condition:         textVal(a.Condition),
	}
}

// --- helpers de conversión pgtype → tipos proto ---

func textVal(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

func int4Val(i pgtype.Int4) int32 {
	if i.Valid {
		return i.Int32
	}
	return 0
}

func numVal(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, err := n.Float64Value()
	if err != nil {
		return 0
	}
	return f.Float64
}

func tsUnix(t pgtype.Timestamp) int64 {
	if t.Valid {
		return t.Time.Unix()
	}
	return 0
}

// splitURLs convierte el campo image_urls (TEXT, URLs separadas por coma) en slice.
func splitURLs(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// newBidID genera un identificador único de puja (Nats-Msg-Id para idempotencia).
func newBidID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
