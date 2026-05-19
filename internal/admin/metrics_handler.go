package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	helixmetrics "helix/internal/metrics"
)

func (h *handlers) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.tracker.Snapshot())
}

func (h *handlers) streamMetrics(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sendSnapshot := func(snap helixmetrics.Snapshot) {
		data, _ := json.Marshal(snap)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	sendSnapshot(h.tracker.Snapshot())

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			sendSnapshot(h.tracker.Snapshot())
		}
	}
}
