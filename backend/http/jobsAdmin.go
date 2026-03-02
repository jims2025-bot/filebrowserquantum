package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jims2025-bot/filebrowserquantum/backend/jobs"
)

// getJobsStatusHandler returns the status of all registered background jobs.
// @Summary List all background jobs
// @Tags Admin
// @Produce json
// @Success 200 {array} jobs.JobStatus
// @Failure 403
// @Router /api/jobs [get]
func getJobsStatusHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.Admin {
		return http.StatusForbidden, fmt.Errorf("admin only")
	}
	return renderJSON(w, r, jobs.GetAll())
}

// runJobHandler triggers a named background job immediately.
// @Summary Manually trigger a background job
// @Tags Admin
// @Param jobname path string true "Job name (heatmap, integrity, iptcindex)"
// @Success 200 {object} HttpResponse
// @Failure 403
// @Failure 404
// @Failure 409 "Job already running"
// @Router /api/jobs/{jobname}/run [post]
func runJobHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.Admin {
		return http.StatusForbidden, fmt.Errorf("admin only")
	}

	jobname := r.PathValue("jobname")
	if jobname == "" {
		return http.StatusBadRequest, fmt.Errorf("jobname is required")
	}

	if err := jobs.RunNow(jobname); err != nil {
		// Distinguish "not found" from "already running"
		if err.Error() == fmt.Sprintf("job '%s' not found", jobname) {
			return http.StatusNotFound, err
		}
		return http.StatusConflict, err // already running
	}

	return renderJSON(w, r, HttpResponse{Message: fmt.Sprintf("Job '%s' started", jobname)})
}

// getJobsStatusPublicHandler is a helper used by getJobsHandler (index info) to avoid naming collision.
// The original getJobsHandler in httpJobs.go handles GET /api/jobs/{action}/{target}.
func init() {
	// Ensure the jobs package is linked
	_ = json.Marshal
}
