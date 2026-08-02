//go:build ignore

package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"

	_ "github.com/lib/pq"
)

func main() {
	db, err := sql.Open("postgres", "postgres://postgres:postgres@localhost:5432/motionmesh?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err) // DB is probably not running, we expect this to fail if docker isn't running
	}

	files, err := filepath.Glob("infra/postgres/migrations/*.sql")
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		fmt.Printf("Running %s...\n", file)
		content, err := ioutil.ReadFile(file)
		if err != nil {
			log.Fatal(err)
		}
		_, err = db.Exec(string(content))
		if err != nil {
			log.Printf("Error running %s: %v", file, err)
		} else {
			fmt.Printf("Successfully ran %s\n", file)
		}
	}
}
