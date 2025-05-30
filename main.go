package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	service := NewIssueService()
	StartSync(service)

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/", indexHandler(service))
	http.HandleFunc("/sync", syncOverviewHandler(service))
	http.HandleFunc("/{key}", issueHandler(service))

	fmt.Println("Starting server...")
	log.Fatal(http.ListenAndServe("localhost:8080", nil))
}
