package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	migrationFile := flag.String("migrate", "", "Run a specific migration file")
	noSync := flag.Bool("nosync", false, "Disable background syncing")
	flag.Parse()

	err := os.MkdirAll("logs", 0755)
	if err != nil {
		log.Fatalf("Failed to create logs directory: %v", err)
	}
	file, err := os.OpenFile("logs/mojira.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	log.SetOutput(io.MultiWriter(os.Stdout, file))

	err = godotenv.Overload()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	service := NewIssueService()
	if *migrationFile != "" {
		if err := service.db.RunMigration(*migrationFile); err != nil {
			log.Fatal(err)
		}
		return
	}
	if !*noSync {
		StartSync(service)
	}

	http.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("User-agent: *\nAllow: /"))
	})
	http.HandleFunc("/static/", staticHandler)
	http.HandleFunc("/", indexHandler(service))
	http.HandleFunc("/sync", syncOverviewHandler(service))
	http.HandleFunc("/{key}", issueHandler(service))

	http.HandleFunc("/api/search", apiSearchHandler(service))
	http.HandleFunc("/api/issues/{key}/refresh", apiRefreshHandler(service))

	log.Println("Starting server...")
	log.Fatal(http.ListenAndServe("localhost:8080", nil))
}
