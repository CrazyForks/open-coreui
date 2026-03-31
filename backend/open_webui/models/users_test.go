package models

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
	"github.com/xxnuo/open-coreui/backend/open_webui/migrations"
)

func openModelsTestDB(t *testing.T) *dbinternal.Handle {
	t.Helper()

	tempDir := t.TempDir()
	db, err := dbinternal.Open(context.Background(), dbinternal.Options{
		DatabaseURL:     "sqlite:///" + filepath.Join(tempDir, "models.db"),
		EnableSQLiteWAL: true,
		OpenTimeout:     5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.Run(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestUsersTableCRUD(t *testing.T) {
	t.Parallel()

	db := openModelsTestDB(t)
	defer db.Close()

	users := NewUsersTable(db)
	inserted, err := users.InsertNewUser(context.Background(), UserInsertParams{
		ID:              "user-1",
		Email:           "dev@example.com",
		Name:            "Dev",
		Role:            "admin",
		ProfileImageURL: "/user.png",
		Settings: map[string]any{
			"ui": map[string]any{"theme": "light"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if inserted == nil || inserted.ID != "user-1" {
		t.Fatal("expected inserted user")
	}

	found, err := users.GetUserByEmail(context.Background(), "DEV@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.ID != "user-1" {
		t.Fatal("expected found user by email")
	}

	newName := "Developer"
	newBio := "bio"
	newTimezone := "Asia/Shanghai"
	updated, err := users.UpdateUserByID(context.Background(), "user-1", UserUpdateParams{
		Name:     &newName,
		Bio:      &newBio,
		Timezone: &newTimezone,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated == nil || updated.Name != newName || updated.Bio != newBio || updated.Timezone != newTimezone {
		t.Fatal("expected updated user fields")
	}

	deleted, err := users.DeleteUserByID(context.Background(), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("expected deleted user")
	}
}
