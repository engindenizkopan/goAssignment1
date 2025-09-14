package transporthttp

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"example.com/goAssignment1/internal/config"
	"example.com/goAssignment1/internal/domain"
	"example.com/goAssignment1/internal/idempotency"
	"example.com/goAssignment1/internal/ingest"
	spg "example.com/goAssignment1/internal/storage/postgres"
)

type ServerDeps struct {
	Cfg      config.Config
	Ingestor *ingest.Ingestor
	DB       *spg.DB
	Now      func() time.Time
}

func decodeJSONStrict(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// --- Health ---

func (d *ServerDeps) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (d *ServerDeps) HandleReadyz(w http.ResponseWriter, r *http.Request) {
	if err := d.DB.Ready(r.Context()); err != nil {
		WriteProblem(w, http.StatusServiceUnavailable, "not ready", "database not reachable", nil)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ready"}`))
}

// --- Events (single) ---

func (d *ServerDeps) HandlePostEvent(w http.ResponseWriter, r *http.Request) {
	defer DrainBody(r)
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var ev domain.Event
	if err := decodeJSONStrict(r, &ev); err != nil {
		WriteProblem(w, http.StatusBadRequest, "invalid json", err.Error(), nil)
		return
	}
	errs := domain.ValidateEvent(&ev, d.Now(), d.Cfg.ClockSkew)
	if len(errs) > 0 {
		prob := map[string][]string{}
		for _, fe := range errs {
			prob[fe.Field] = append(prob[fe.Field], fe.Msg)
		}
		WriteProblem(w, http.StatusBadRequest, "validation failed", "one or more fields are invalid", prob)
		return
	}
	_, _ = idempotency.DeriveKey(&ev)

	if ok := d.Ingestor.Enqueue(ev); !ok {
		WriteProblem(w, http.StatusServiceUnavailable, "overloaded", "ingest queue is full, please retry", nil)
		return
	}
	log.Printf("[api] queued 1 event: name=%s user=%s ts=%d", ev.EventName, ev.UserID, ev.Timestamp)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"queued"}`))
}

// --- Events (bulk) ---

type bulkReq struct {
	Events []domain.Event `json:"events"`
}

func (d *ServerDeps) HandlePostEventsBulk(w http.ResponseWriter, r *http.Request) {
	defer DrainBody(r)
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var br bulkReq
	if err := decodeJSONStrict(r, &br); err != nil {
		WriteProblem(w, http.StatusBadRequest, "invalid json", err.Error(), nil)
		return
	}
	ptrs := make([]*domain.Event, len(br.Events))
	for i := range br.Events {
		ptrs[i] = &br.Events[i]
	}
	if all, top := domain.ValidateBulk(ptrs, 100, d.Now(), d.Cfg.ClockSkew); top != nil {
		prob := map[string][]string{}
		for i, arr := range all {
			if len(arr) == 0 {
				continue
			}
			k := "events[" + strconv.Itoa(i) + "]"
			for _, fe := range arr {
				prob[k+"."+fe.Field] = append(prob[k+"."+fe.Field], fe.Msg)
			}
		}
		WriteProblem(w, http.StatusBadRequest, "validation failed", top.Error(), prob)
		return
	}
	for _, ev := range br.Events {
		if ok := d.Ingestor.Enqueue(ev); !ok {
			WriteProblem(w, http.StatusServiceUnavailable, "overloaded", "ingest queue is full, please retry", nil)
			return
		}
	}
	log.Printf("[api] queued %d events (bulk)", len(br.Events))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"accepted_count":` + strconv.Itoa(len(br.Events)) + `}`))
}

// --- Metrics ---

type metricsTotals struct {
	Count       int64 `json:"count"`
	UniqueUsers int64 `json:"unique_users"`
}
type metricsBucket struct {
	BucketStart int64 `json:"bucket_start"`
	Count       int64 `json:"count"`
	UniqueUsers int64 `json:"unique_users"`
}
type metricsResp struct {
	Totals  metricsTotals   `json:"totals"`
	Buckets []metricsBucket `json:"buckets"`
}

const defaultWindowSeconds = int64(24 * 60 * 60)  // last 24h default
const maxWindowSeconds = int64(90 * 24 * 60 * 60) // cap at 90 days (guardrail)

func (d *ServerDeps) HandleGetMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	eventName := strings.TrimSpace(q.Get("event_name")) // optional
	fromStr := q.Get("from")                            // optional
	toStr := q.Get("to")                                // optional
	groupBy := q.Get("group_by")
	channel := strings.TrimSpace(q.Get("channel"))

	now := d.Now().Unix()
	var from, to int64
	var err error

	switch {
	case fromStr == "" && toStr == "":
		from, to = now-defaultWindowSeconds, now
	case fromStr != "" && toStr == "":
		from, err = strconv.ParseInt(fromStr, 10, 64)
		if err != nil {
			WriteProblem(w, http.StatusBadRequest, "invalid parameters", "from must be epoch seconds", nil)
			return
		}
		to = now
	case fromStr == "" && toStr != "":
		to, err = strconv.ParseInt(toStr, 10, 64)
		if err != nil {
			WriteProblem(w, http.StatusBadRequest, "invalid parameters", "to must be epoch seconds", nil)
			return
		}
		from = to - defaultWindowSeconds
	default:
		from, err = strconv.ParseInt(fromStr, 10, 64)
		if err != nil {
			WriteProblem(w, http.StatusBadRequest, "invalid parameters", "from must be epoch seconds", nil)
			return
		}
		to, err = strconv.ParseInt(toStr, 10, 64)
		if err != nil {
			WriteProblem(w, http.StatusBadRequest, "invalid parameters", "to must be epoch seconds", nil)
			return
		}
	}

	// guardrail: cap excessively large ranges
	if to-from > maxWindowSeconds {
		from = to - maxWindowSeconds
	}

	// optional filters
	var evPtr *string
	if eventName != "" {
		evPtr = &eventName
	}
	var chPtr *string
	if channel != "" {
		chPtr = &channel
	}

	ctx := r.Context()
	tot, err := d.DB.QueryTotals(ctx, evPtr, from, to, chPtr)
	if err != nil {
		WriteProblem(w, http.StatusInternalServerError, "query error", err.Error(), nil)
		return
	}

	var resp metricsResp
	resp.Totals = metricsTotals{Count: tot.Count, UniqueUsers: tot.UniqueUsers}

	if groupBy == "day" {
		bs, err := d.DB.QueryBucketsDaily(ctx, evPtr, from, to, chPtr)
		if err != nil {
			WriteProblem(w, http.StatusInternalServerError, "query error", err.Error(), nil)
			return
		}
		for _, b := range bs {
			resp.Buckets = append(resp.Buckets, metricsBucket{BucketStart: b.BucketStart, Count: b.Count, UniqueUsers: b.UniqueUsers})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// --- Serve OpenAPI (convenience) ---

func (d *ServerDeps) HandleOpenAPI(w http.ResponseWriter, r *http.Request) {
	wd, _ := os.Getwd()
	p := filepath.Join(wd, "api", "openapi.yaml")
	http.ServeFile(w, r, p)
}

// --- Router ---

func (d *ServerDeps) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", d.HandleHealthz)
	mux.HandleFunc("/readyz", d.HandleReadyz)
	mux.HandleFunc("/openapi.yaml", d.HandleOpenAPI)

	var postEvent http.Handler = http.HandlerFunc(d.HandlePostEvent)
	postEvent = BodyLimit(d.Cfg.MaxBodyBytes)(postEvent)
	postEvent = RequireJSON(postEvent)
	postEvent = APIKeyAuth(d.Cfg.APIKeys)(postEvent)
	mux.Handle("/events", postEvent)

	var postBulk http.Handler = http.HandlerFunc(d.HandlePostEventsBulk)
	postBulk = BodyLimit(d.Cfg.MaxBodyBytes)(postBulk)
	postBulk = RequireJSON(postBulk)
	postBulk = APIKeyAuth(d.Cfg.APIKeys)(postBulk)
	mux.Handle("/events/bulk", postBulk)

	var getMetrics http.Handler = http.HandlerFunc(d.HandleGetMetrics)
	getMetrics = RateLimitPerMinute(d.Cfg.RateLimitMetricsPerMin, d.Now)(getMetrics)
	getMetrics = APIKeyAuth(d.Cfg.APIKeys)(getMetrics)
	mux.Handle("/metrics", getMetrics)

	return mux
}
