package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	readerv1 "github.com/amusaev/url-shortener/libs/gen/reader/v1"
	writerv1 "github.com/amusaev/url-shortener/libs/gen/writer/v1"
)

// Handlers serves the REST and redirect endpoints.
type Handlers struct {
	writer writerv1.WriterServiceClient
	reader readerv1.ReaderServiceClient
	logger *slog.Logger
}

// NewHandlers builds the gateway handlers.
func NewHandlers(writer writerv1.WriterServiceClient, reader readerv1.ReaderServiceClient, logger *slog.Logger) *Handlers {
	return &Handlers{writer: writer, reader: reader, logger: logger}
}

type createRequest struct {
	LongURL     string     `json:"long_url"`
	CustomAlias string     `json:"custom_alias,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

type urlResponse struct {
	Code      string     `json:"code"`
	ShortURL  string     `json:"short_url,omitempty"`
	LongURL   string     `json:"long_url"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Root reports basic service liveness on the public port.
func (h *Handlers) Root(w http.ResponseWriter, r *http.Request) {
	writeJSON(r.Context(), w, h.logger, http.StatusOK, map[string]string{
		"service": "api-gateway",
		"status":  "ok",
	})
}

// Create handles POST /api/v1/urls.
func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req createRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeClientError(ctx, w, h.logger, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	grpcReq := &writerv1.ShortenRequest{LongUrl: req.LongURL}
	if req.CustomAlias != "" {
		alias := req.CustomAlias
		grpcReq.CustomAlias = &alias
	}
	if req.ExpiresAt != nil {
		grpcReq.ExpiresAt = timestamppb.New(*req.ExpiresAt)
	}

	resp, err := h.writer.Shorten(ctx, grpcReq)
	if err != nil {
		writeError(ctx, w, h.logger, err)
		return
	}

	writeJSON(ctx, w, h.logger, http.StatusCreated, urlResponse{
		Code:      resp.GetCode(),
		ShortURL:  resp.GetShortUrl(),
		LongURL:   resp.GetLongUrl(),
		ExpiresAt: protoTime(resp.GetExpiresAt()),
	})
}

// Metadata handles GET /api/v1/urls/{code}, returning mapping details as JSON.
func (h *Handlers) Metadata(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	code := r.PathValue("code")

	resp, err := h.reader.Resolve(ctx, &readerv1.ResolveRequest{Code: code})
	if err != nil {
		writeError(ctx, w, h.logger, err)
		return
	}

	writeJSON(ctx, w, h.logger, http.StatusOK, urlResponse{
		Code:      code,
		LongURL:   resp.GetLongUrl(),
		ExpiresAt: protoTime(resp.GetExpiresAt()),
	})
}

// Redirect handles GET /{code}, issuing a 302 to the long URL.
func (h *Handlers) Redirect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	code := r.PathValue("code")

	resp, err := h.reader.Resolve(ctx, &readerv1.ResolveRequest{Code: code})
	if err != nil {
		writeError(ctx, w, h.logger, err)
		return
	}

	http.Redirect(w, r, resp.GetLongUrl(), http.StatusFound)
}

func protoTime(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	return &t
}
