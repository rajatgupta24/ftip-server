package main

import (
	"fmt"
	"net/http"
)

func main() {
	fmt.Println("Hello, 世界")

	mux := http.NewServeMux()
	wrappedMux := logger(mux)

	mux.HandleFunc("/v1/healthcheck", healthcheckHandler)
	mux.HandleFunc("/aboutme", aboutme)

	server := &http.Server{
		Addr:    ":5000",
		Handler: wrappedMux,
	}

	fmt.Println("Starting server on :5000")
	server.ListenAndServe()
}

func healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello, server is healthy!")
}

func aboutme(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello, I am a Go web server!")
}

func logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("%s - %s %s\n", r.RemoteAddr, r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
