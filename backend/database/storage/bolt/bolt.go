package bolt

import (
	storm "github.com/asdine/storm/v3"

	"github.com/jims2025-bot/filebrowserquantum/backend/auth"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/share"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/users"
)

// NewStorage creates a storage.Storage based on Bolt DB.
func NewStorage(db *storm.DB) (*auth.Storage, *users.Storage, *share.Storage, *settings.Storage, error) {
	userStore := users.NewStorage(usersBackend{db: db})
	shareStore := share.NewStorage(shareBackend{db: db})
	settingsStore := settings.NewStorage(settingsBackend{db: db})
	authStore, err := auth.NewStorage(authBackend{db: db}, userStore)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return authStore, userStore, shareStore, settingsStore, nil
}
