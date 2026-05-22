package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, ok := s.jm.Get(id)
	if !ok {
		http.Error(w, "backtest not found", http.StatusNotFound)
		return
	}

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("ws accept: %v", err)
		return
	}
	defer c.CloseNow()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pingTicker.C:
			if err := c.Ping(ctx); err != nil {
				return
			}
		case msg, ok := <-job.TickChan:
			if !ok {
				// channel closed — job finished
				c.Close(websocket.StatusNormalClosure, "")
				return
			}
			data, err := json.Marshal(msg)
			if err != nil {
				log.Printf("ws marshal: %v", err)
				continue
			}
			if err := c.Write(ctx, websocket.MessageText, data); err != nil {
				return
			}
		}
	}
}
