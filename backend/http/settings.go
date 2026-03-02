package http

import (
	"encoding/json"
	"net/http"

	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
)

// settingsGetHandler retrieves the current system settings.
// @Summary Get system settings
// @Description Returns the current configuration settings for signup, user directories, rules, frontend.
// @Tags Settings
// @Accept json
// @Produce json
// @Param property query string false "Property to retrieve: `userDefaults`, `frontend`, `auth`, `server`, `sources`"
// @Success 200 {object} settings.Settings "System settings data"
// @Router /api/settings [get]
func settingsGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	property := r.URL.Query().Get("property")
	if property != "" {
		// get property by name
		switch property {
		case "userDefaults":
			return renderJSON(w, r, config.UserDefaults)
		case "frontend":
			return renderJSON(w, r, config.Frontend)
		case "auth":
			return renderJSON(w, r, config.Auth)
		case "server":
			return renderJSON(w, r, config.Server)
		case "sources":
			return renderJSON(w, r, config.Server.Sources)
		default:
			return http.StatusNotFound, nil
		}
	}
	return renderJSON(w, r, config)
}

// settingsPutHandler updates the system settings.
// @Summary Update system settings
// @Description Updates the system configuration.
// @Tags Settings
// @Accept json
// @Produce json
// @Param settings body settings.Settings true "Updated settings data"
// @Success 200 {object} HttpResponse
// @Failure 403 "Admin only"
// @Router /api/settings [put]
func settingsPutHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	var newSettings settings.Settings
	if err := json.NewDecoder(r.Body).Decode(&newSettings); err != nil {
		return http.StatusBadRequest, err
	}

	if err := settings.Update(&newSettings); err != nil {
		return http.StatusInternalServerError, err
	}

	return renderJSON(w, r, HttpResponse{Message: "Settings updated successfully"})
}
