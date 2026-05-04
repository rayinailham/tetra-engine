package main

import (
	"log"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	db, err := sqlx.Connect("postgres", "host=127.0.0.1 port=5432 user=postgres password=Anjay123 dbname=tetra sslmode=disable")
	if err != nil {
		log.Fatalln(err)
	}
	defer db.Close()

	res, err := db.Exec("UPDATE orders SET status = 'NO RECOMMENDATION' WHERE status = 'ERROR';")
	if err != nil {
		log.Fatalln(err)
	}

	affected, _ := res.RowsAffected()
	log.Printf("Updated %d rows", affected)
}
