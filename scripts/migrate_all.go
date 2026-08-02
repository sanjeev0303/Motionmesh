//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	migrations := []string{
		"scripts/migrations/000_init_schema.sql",
		"scripts/migrations/001_reconcile_architecture.sql",
		"scripts/migrations/002_video_chapters_and_progress.sql",
		"scripts/migrations/004_drop_playlists.sql",
		"scripts/migrations/005_drop_analytics.sql",
		"infra/postgres/migrations/003_external_user_id.sql",
		"infra/postgres/migrations/004_indexes.sql",
		"infra/postgres/migrations/009_account_balance.sql",
	}

	for _, m := range migrations {
		b, err := os.ReadFile(m)
		if err != nil {
			panic(fmt.Sprintf("Failed to read %s: %v", m, err))
		}
		
		_, err = db.Exec(ctx, string(b))
		if err != nil {
			panic(fmt.Sprintf("Failed to execute %s: %v", m, err))
		}
		fmt.Printf("Applied %s\n", m)
	}
}
