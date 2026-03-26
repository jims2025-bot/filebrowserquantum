package http

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
)

type UsageEvent struct {
	Username string    `json:"username"`
	Datetime time.Time `json:"datetime"`
	MapUsed  bool      `json:"mapUsed"`
}

var usageMu sync.Mutex
const usageFile = "usage_logs.json"

func getUsageEvents() ([]UsageEvent, error) {
	usageMu.Lock()
	defer usageMu.Unlock()

	data, err := os.ReadFile(usageFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []UsageEvent{}, nil
		}
		return nil, err
	}

	var events []UsageEvent
	if err := json.Unmarshal(data, &events); err != nil {
		// Log and reset if corrupted
		logger.Errorf("Failed to parse usage logs: %v", err)
		return []UsageEvent{}, nil
	}
	return events, nil
}

func saveUsageEvents(events []UsageEvent) error {
	usageMu.Lock()
	defer usageMu.Unlock()

	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(usageFile, data, 0644)
}

func LogUsageEvent(username string) {
	events, err := getUsageEvents()
	if err != nil {
		logger.Errorf("Error reading usage events: %v", err)
		events = []UsageEvent{}
	}

	// Data Retention: Keep only entries from the last year (365 days)
	oneYearAgo := time.Now().AddDate(-1, 0, 0)
	filtered := make([]UsageEvent, 0, len(events))
	for _, e := range events {
		if e.Datetime.After(oneYearAgo) {
			filtered = append(filtered, e)
		}
	}
	events = filtered

	// Hard limit to prevent extreme file size (e.g. 50,000 events)
	if len(events) > 50000 {
		events = events[len(events)-50000:]
	}

	events = append(events, UsageEvent{
		Username: username,
		Datetime: time.Now(),
		MapUsed:  false,
	})

	if err := saveUsageEvents(events); err != nil {
		logger.Errorf("Error saving usage events: %v", err)
	}
}

func UpdateLatestUsageMapFlag(username string) {
	events, err := getUsageEvents()
	if err != nil {
		logger.Errorf("Error reading usage events: %v", err)
		return
	}

	// Reverse iterate to find the most recent login for this user
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Username == username {
			if events[i].MapUsed {
				return // Already flagged, skip saving
			}
			events[i].MapUsed = true
			if err := saveUsageEvents(events); err != nil {
				logger.Errorf("Error saving usage events: %v", err)
			}
			return
		}
	}
}

func getUsageHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	events, err := getUsageEvents()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	
	total := len(events)
	
	// Reverse list so newest is on top
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}

	// Pagination: 10 per page
	limit := 10
	pageStr := r.URL.Query().Get("page")
	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	start := (page - 1) * limit
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	paginatedEvents := events[start:end]

	logger.Infof("Usage: Returning %d logs (Page %d, Total %d, Limit %d) for user %s", len(paginatedEvents), page, total, limit, d.user.Username)

	return renderJSON(w, r, map[string]interface{}{
		"total": total,
		"page":  page,
		"limit": limit,
		"logs":  paginatedEvents,
	})
}

func markMapUsedHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if d.user != nil {
		UpdateLatestUsageMapFlag(d.user.Username)
	}
	return renderJSON(w, r, map[string]interface{}{"status": "ok"})
}
