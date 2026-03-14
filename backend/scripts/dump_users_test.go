package main

import (
	"testing"
	"fmt"
	"github.com/jims2025-bot/filebrowserquantum/backend/database"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/storage"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
)

func TestDump(t *testing.T) {
	settings.Initialize("config.yaml")

	db, err := database.New(settings.Config.Server.Database)
	if err != nil {
		fmt.Printf("Error opening db: %v\n", err)
		return
	}
	defer db.Close()
	store := storage.NewStorage(db)
	users, _ := store.Users.Gets()
	for _, u := range users {
		fmt.Printf("User: %s\n", u.Username)
		for i, s := range u.Scopes {
			fmt.Printf("  Scope %d: Name='%s', Scope='%s', Alias='%s'\n", i, s.Name, s.Scope, s.Alias)
		}
	}
}
