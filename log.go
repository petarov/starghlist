package main

import (
	"log"
	"net/http"
)

func logInit() {
	log.SetFlags(log.Ldate | log.Ltime)
}

// type responseWriter struct {
// 	http.ResponseWriter
// 	statusCode int
// }

// func (rw *responseWriter) WriteHeader(code int) {
// 	rw.statusCode = code
// 	rw.ResponseWriter.WriteHeader(code)
// }

// func loggingHandler(handler http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		start := time.Now()

// 		// Create a response writer wrapper to capture status code.
// 		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

// 		// Log request details.
// 		log.Printf("[REQUEST] %s | %s | %s %s",
// 			start.Format(time.RFC3339),
// 			r.RemoteAddr,
// 			r.Method,
// 			r.URL.Path)

// 		// Call the actual handler.
// 		handler.ServeHTTP(wrapped, r)

// 		// Log response details.
// 		duration := time.Since(start)
// 		log.Printf("[RESPONSE] %s | %s | %s %s | Status: %d | Duration: %v",
// 			time.Now().Format(time.RFC3339),
// 			r.RemoteAddr,
// 			r.Method,
// 			r.URL.Path,
// 			wrapped.statusCode,
// 			duration)
// 	})
// }

func logRequest(r *http.Request) {
	log.Printf("%s \"%s %s\" \"%s\"",
		r.RemoteAddr,
		r.Method,
		r.URL.Path,
		r.UserAgent())
}
