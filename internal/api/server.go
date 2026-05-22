package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/felix/papertrading/internal/app"
	"github.com/felix/papertrading/internal/domain/metrics"
	"github.com/felix/papertrading/internal/domain/order"
	"github.com/felix/papertrading/internal/domain/portfolio"
)

type Server struct {
	jm *JobManager
}

func NewServer(jm *JobManager) *Server {
	return &Server{jm: jm}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/backtests", s.handleCreate)
	mux.HandleFunc("GET /api/v1/backtests", s.handleList)
	mux.HandleFunc("GET /api/v1/backtests/{id}", s.handleGet)
	mux.HandleFunc("DELETE /api/v1/backtests/{id}", s.handleCancel)
	return withLogging(mux)
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateBacktestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if len(req.Symbols) == 0 {
		writeError(w, http.StatusBadRequest, "symbols is required")
		return
	}
	if len(req.CSVPaths) == 0 {
		writeError(w, http.StatusBadRequest, "csv_paths is required")
		return
	}
	if req.InitialCash == "" {
		req.InitialCash = "100000.00"
	}
	if req.Strategy.Name == "" {
		req.Strategy.Name = "ma_cross"
	}
	if req.Strategy.Params == nil {
		req.Strategy.Params = make(map[string]any)
	}

	job := s.jm.Create(req)
	s.jm.Run(job)

	resp := toResponse(job)
	writeJSON(w, http.StatusAccepted, resp)
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	jobs := s.jm.List()
	ids := make([]string, len(jobs))
	for i, j := range jobs {
		ids[i] = j.ID
	}
	writeJSON(w, http.StatusOK, ListResponse{Backtests: ids})
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, ok := s.jm.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "backtest not found")
		return
	}
	writeJSON(w, http.StatusOK, toResponse(job))
}

func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.jm.Cancel(id) {
		writeError(w, http.StatusNotFound, "backtest not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func toResponse(job *BacktestJob) BacktestResponse {
	resp := BacktestResponse{
		ID:        job.ID,
		Status:    string(job.Status),
		CreatedAt: job.CreatedAt.Format(time.RFC3339),
	}
	if !job.CompletedAt.IsZero() {
		resp.CompletedAt = job.CompletedAt.Format(time.RFC3339)
	}
	if job.Error != "" {
		resp.Error = job.Error
	}
	if job.Result != nil {
		resp.Result = toSimulationDTO(job.Result)
	}
	return resp
}

func toSimulationDTO(r *app.SimulationResult) *SimulationDTO {
	return &SimulationDTO{
		Portfolio:   toPortfolioDTO(r.Portfolio),
		Orders:      toOrderDTOs(r.Orders),
		Fills:       toFillDTOs(r.Fills),
		EquityCurve: toEquityCurveDTO(r.EquityCurve),
		Metrics:     toMetricsDTO(&r.Metrics),
		StartTime:   r.StartTime.Format(time.RFC3339),
		EndTime:     r.EndTime.Format(time.RFC3339),
	}
}

func toPortfolioDTO(p *portfolio.Portfolio) *PortfolioDTO {
	dto := &PortfolioDTO{
		ID:           string(p.ID),
		Cash:         p.Cash.String(),
		Positions:    make(map[string]*PositionDTO),
		ClosedTrades: make([]ClosedTradeDTO, 0, len(p.ClosedTrades())),
		RealizedPnL:  p.RealizedPnL().String(),
		CreatedAt:    p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    p.UpdatedAt.Format(time.RFC3339),
	}
	for sym, pos := range p.Positions {
		dto.Positions[string(sym)] = &PositionDTO{
			Symbol:        string(sym),
			Quantity:      pos.Quantity,
			AvgEntryPrice: pos.AvgEntryPrice.String(),
			OpenedAt:      pos.OpenedAt.Format(time.RFC3339),
		}
	}
	for _, ct := range p.ClosedTrades() {
		dto.ClosedTrades = append(dto.ClosedTrades, ClosedTradeDTO{
			Symbol:     string(ct.Symbol),
			EntryPrice: ct.EntryPrice.String(),
			ExitPrice:  ct.ExitPrice.String(),
			Quantity:   ct.Quantity,
			PnL:        ct.PnL.String(),
			OpenedAt:   ct.OpenedAt.Format(time.RFC3339),
			ClosedAt:   ct.ClosedAt.Format(time.RFC3339),
		})
	}
	return dto
}

func toOrderDTOs(orders []*order.Order) []OrderDTO {
	dtos := make([]OrderDTO, len(orders))
	for i, o := range orders {
		dtos[i] = OrderDTO{
			ID:          string(o.ID),
			PortfolioID: o.PortfolioID,
			Symbol:      string(o.Symbol),
			Side:        o.Side.String(),
			OrderType:   o.Type.String(),
			Status:      o.Status.String(),
			Price:       o.Price.String(),
			StopPrice:   o.StopPrice.String(),
			Quantity:    o.Quantity,
			FilledQty:   o.FilledQty,
			CreatedAt:   o.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   o.UpdatedAt.Format(time.RFC3339),
			Fills:       toFillDTOs(o.Fills),
		}
	}
	return dtos
}

func toFillDTOs(fills []order.Fill) []FillDTO {
	if fills == nil {
		return nil
	}
	dtos := make([]FillDTO, len(fills))
	for i, f := range fills {
		dtos[i] = FillDTO{
			ID:         string(f.ID),
			OrderID:    string(f.OrderID),
			Price:      f.Price.String(),
			Quantity:   f.Quantity,
			Commission: f.Commission.String(),
			Timestamp:  f.Timestamp.Format(time.RFC3339),
		}
	}
	return dtos
}

func toEquityCurveDTO(curve []app.EquityPoint) []EquityPointDTO {
	dtos := make([]EquityPointDTO, len(curve))
	for i, ep := range curve {
		dtos[i] = EquityPointDTO{
			Time:   ep.Time.Format(time.RFC3339),
			Equity: ep.Equity.String(),
		}
	}
	return dtos
}

func toMetricsDTO(m *metrics.Metrics) *MetricsDTO {
	return &MetricsDTO{
		TotalPnL:      m.TotalPnL.String(),
		MaxDrawdown:   m.MaxDrawdown,
		SharpeRatio:   m.SharpeRatio,
		WinRate:       m.WinRate,
		ProfitFactor:  m.ProfitFactor,
		TotalTrades:   m.TotalTrades,
		WinningTrades: m.WinningTrades,
		LosingTrades:  m.LosingTrades,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		log.Printf("%s %s done in %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func Start(addr string, jm *JobManager) error {
	srv := NewServer(jm)
	log.Printf("API server listening on %s", addr)
	return http.ListenAndServe(addr, srv.Handler())
}
