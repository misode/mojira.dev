package main

import (
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
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

func InstrumentMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rr := &responseRecorder{ResponseWriter: w, statusCode: 200}
		next.ServeHTTP(rr, r)
		duration := time.Since(start).Seconds()
		routePattern := chi.RouteContext(r.Context()).RoutePattern()
		httpDuration.WithLabelValues(routePattern, r.Method, http.StatusText(rr.statusCode)).Observe(duration)
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

	r := chi.NewRouter()
	r.Use(middleware.RedirectSlashes)

	r.Get("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("User-agent: *\nAllow: /"))
	})
	r.Get("/metrics", metricsHandler)
	r.Get("/static/*", staticHandler)

	r.Get("/browse/{key}", issueRedirectHandler)
	r.Get("/browse/{project}/issues/{key}", issueRedirectHandler)

	r.Group(func(r chi.Router) {
		r.Use(httprate.Limit(300, time.Minute, httprate.WithKeyFuncs(KeyByCFConnectingIP)))
		r.Use(InstrumentMiddleware)

		r.Get("/", indexHandler(service))
		r.Get("/queue", queueOverviewHandler(service))
		r.Get("/{key}", issueHandler(service))
		r.Get("/user/{name}", userHandler(service))

		r.Post("/api/search", apiSearchHandler(service))
		r.Get("/api/issues/{key}/refresh", apiRefreshHandler(service))
		r.Get("/api/user/{name}/load-more", apiUserLoadMoreHandler(service))

		r.Get("/api/v1/issues/{key}", apiV1Issue(service))
	})

	log.Println("Starting server...")
	log.Fatal(http.ListenAndServe("localhost:8080", r))
}

func KeyByCFConnectingIP(r *http.Request) (string, error) {
	var ip string
	if cfip := r.Header.Get("CF-Connecting-IP"); cfip != "" {
		ip = cfip
	} else {
		var err error
		ip, _, err = net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
	}
	return canonicalizeIP(ip), nil
}

// From httprate, but not public
func canonicalizeIP(ip string) string {
	isIPv6 := false
	for i := 0; !isIPv6 && i < len(ip); i++ {
		switch ip[i] {
		case '.':
			return ip
		case ':':
			isIPv6 = true
		}
	}
	if !isIPv6 {
		return ip
	}
	ipv6 := net.ParseIP(ip)
	if ipv6 == nil {
		return ip
	}
	return ipv6.Mask(net.CIDRMask(64, 128)).String()
}
