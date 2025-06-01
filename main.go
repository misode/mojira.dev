package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	noSync := flag.Bool("nosync", false, "Disable background syncing")
	flag.Parse()

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	service := NewIssueService()
	if !*noSync {
		StartSync(service)
	}

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/", indexHandler(service))
	http.HandleFunc("/sync", syncOverviewHandler(service))
	http.HandleFunc("/{key}", issueHandler(service))

	http.HandleFunc("/api/search", apiSearchHandler(service))

	fmt.Println("Starting server...")
	log.Fatal(http.ListenAndServe("localhost:8080", nil))
}
