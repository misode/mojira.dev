package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "http_request_duration_seconds",
	Help:    "Duration of HTTP requests by handler, method and code",
	Buckets: prometheus.DefBuckets,
}, []string{"handler", "method", "code"})

func instrumentHandler(name string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rr := &responseRecorder{ResponseWriter: w, statusCode: 200}
		next.ServeHTTP(rr, r)
		duration := time.Since(start).Seconds()
		httpDuration.WithLabelValues(name, r.Method, http.StatusText(rr.statusCode)).Observe(duration)
	})
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *responseRecorder) WriteHeader(code int) {
	if r.statusCode == 0 {
		r.statusCode = code
		r.ResponseWriter.WriteHeader(code)
	}
}

func handle(path string, handler http.Handler) {
	http.Handle(path, instrumentHandler(path, handler))
}

func main() {
	migrationFile := flag.String("migrate", "", "Run a specific migration file")
	noSync := flag.Bool("nosync", false, "Disable background syncing")
	flag.Parse()

	err := godotenv.Overload()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	fileLogger := NewFileLogger("mojira.log")
	lokiLogger := NewLokiLogger(8 * time.Second)
	log.SetOutput(io.MultiWriter(os.Stdout, fileLogger, lokiLogger))

	service := NewIssueService()
	if *migrationFile != "" {
		if err := service.db.RunMigration(*migrationFile); err != nil {
			log.Fatal(err)
		}
		return
	}
	StartSync(service, *noSync)

	http.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("User-agent: *\nAllow: /"))
	})
	http.HandleFunc("/static/", staticHandler)

	http.HandleFunc("/browse/{key}", issueRedirectHandler)
	http.HandleFunc("/browse/{project}/issues/{key}", issueRedirectHandler)

	handle("/", indexHandler(service))
	handle("/queue", queueOverviewHandler(service))
	handle("/{key}", issueHandler(service))
	handle("/user/{name}", userHandler(service))

	handle("/api/search", apiSearchHandler(service))
	handle("/api/issues/{key}/refresh", apiRefreshHandler(service))

	http.HandleFunc("/metrics", metricsHandler)

	log.Println("Starting server...")
	log.Fatal(http.ListenAndServe("localhost:8080", nil))
}
