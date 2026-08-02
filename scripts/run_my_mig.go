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

	b, err := os.ReadFile("infra/postgres/migrations/009_account_balance.sql")
	if err != nil {
		panic(fmt.Sprintf("Failed to read: %v", err))
	}
	
	_, err = db.Exec(ctx, string(b))
	if err != nil {
		panic(fmt.Sprintf("Failed to execute: %v", err))
	}
	fmt.Printf("Applied migration successfully\n")
}
