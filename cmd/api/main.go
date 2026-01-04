package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"database/sql"

	_ "github.com/lib/pq"
)

// 1. Move the variable declaration to the top (but don't initialize it yet)
var db *sql.DB // Package-level variable
var googleOauthConfig *oauth2.Config

const randomState = "random_state_string" // Protects against CSRF

func main() {
	fmt.Println("Hello, 世界")

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	connStr := "postgres://postgres:mysecretpassword@localhost:5432/postgres?sslmode=disable"

	// 3. Open the connection
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Error opening DB:", err)
	}
	defer db.Close()

	// 4. Ping the DB to ensure it's actually reachable
	err = db.Ping()
	if err != nil {
		log.Fatal("System Failure: Could not reach Postgres container!", err)
	}

	fmt.Println("DATABASE CONNECTED! System is now State-Aware.")

	// Global config (In production, move this to an 'auth' package)
	googleOauthConfig = &oauth2.Config{
		RedirectURL:  "http://localhost:5000/auth/google/callback",
		ClientID:     os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
		Endpoint:     google.Endpoint,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("api/v1/healthcheck", healthcheckHandler)

	mux.HandleFunc("/auth/google/login", handleGoogleLogin)
	mux.HandleFunc("/auth/google/callback", googleCallbackHandler)

	wrappedMux := logger(mux)

	fmt.Println("Starting server on :5000")
	log.Fatal(http.ListenAndServe(":5000", wrappedMux))
}

func healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello, server is healthy!")
}

// Redirects user to Google
func handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	url := googleOauthConfig.AuthCodeURL(randomState)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func googleCallbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("state") != randomState {
		http.Error(w, "State mismatch", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	token, err := googleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "Code exchange failed", http.StatusInternalServerError)
		return
	}

	// Fetch User Info
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		http.Error(w, "User info fetch failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var profile map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&profile)

	fmt.Fprintf(w, "Success! Welcome: %s", profile["name"])

	// 1. Prepare the SQL Statement
	// This checks if the email exists. If yes, it updates the name. If no, it creates a new row.
	query := `
		INSERT INTO users (email, name, google_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (email) 
		DO UPDATE SET name = EXCLUDED.name, google_id = EXCLUDED.google_id
		RETURNING id
	`

	var userID int
	err = db.QueryRow(query, profile["email"], profile["name"], profile["id"]).Scan(&userID)

	if err != nil {
		log.Printf("Database Error: %v", err)
		http.Error(w, "Failed to save user", http.StatusInternalServerError)
		return
	}

	fmt.Printf("Successfully synced user %s with internal ID %d\n", profile["email"], userID)
	fmt.Fprintf(w, " Welcome back, %s! Your internal ID is %d", profile["name"], userID)
}

func logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("%s - %s %s\n", r.RemoteAddr, r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
