package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Config struct {
	Port       string
	MongoURI   string
	Database   string
	Collection string
	Allowed    map[string]struct{}
}

type Server struct {
	collection *mongo.Collection
	config     Config
}

type LeadRequest struct {
	FullName string `json:"fullName"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	School   string `json:"school"`
	City     string `json:"city"`
	Subjects string `json:"subjects"`
	Classes  string `json:"classes"`
	Source   string `json:"source"`
}

type LeadDocument struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	FullName  string             `bson:"fullName" json:"fullName"`
	Phone     string             `bson:"phone" json:"phone"`
	Email     string             `bson:"email" json:"email"`
	School    string             `bson:"school,omitempty" json:"school,omitempty"`
	City      string             `bson:"city,omitempty" json:"city,omitempty"`
	Subjects  string             `bson:"subjects,omitempty" json:"subjects,omitempty"`
	Classes   string             `bson:"classes,omitempty" json:"classes,omitempty"`
	Source    string             `bson:"source" json:"source"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type successResponse struct {
	Message string `json:"message"`
	ID      string `json:"id"`
}

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: No .env file found, using system environment variables: %v", err)
	}

	cfg := loadConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatalf("failed to connect to MongoDB: %v", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("failed to ping MongoDB: %v", err)
	}

	server := &Server{
		collection: client.Database(cfg.Database).Collection(cfg.Collection),
		config:     cfg,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.handleHealth)
	mux.HandleFunc("POST /api/leads", server.handleCreateLead)
	mux.HandleFunc("OPTIONS /api/leads", server.handleOptions)

	handler := server.withCORS(mux)

	log.Printf("backend listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

func loadConfig() Config {
	port := getEnv("PORT", "8080")
	mongoURI := strings.TrimSpace(os.Getenv("MONGODB_URI"))
	if mongoURI == "" {
		log.Fatal("MONGODB_URI is required")
	}

	return Config{
		Port:       port,
		MongoURI:   mongoURI,
		Database:   getEnv("MONGODB_DATABASE", "dexworkshop"),
		Collection: getEnv("MONGODB_COLLECTION", "workshop_leads"),
		Allowed:    parseAllowedOrigins(getEnv("ALLOWED_ORIGINS", "http://localhost:3000")),
	}
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func parseAllowedOrigins(raw string) map[string]struct{} {
	origins := make(map[string]struct{})
	for _, origin := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			origins[trimmed] = struct{}{}
		}
	}
	return origins
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			if _, ok := s.config.Allowed[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}
		}

		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleOptions(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCreateLead(w http.ResponseWriter, r *http.Request) {
	var req LeadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	lead, err := normalizeLead(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	result, err := s.collection.InsertOne(ctx, lead)
	if err != nil {
		log.Printf("insert failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to store lead")
		return
	}

	insertedID, _ := result.InsertedID.(primitive.ObjectID)
	writeJSON(w, http.StatusCreated, successResponse{
		Message: "Lead stored successfully",
		ID:      insertedID.Hex(),
	})
}

func normalizeLead(req LeadRequest) (LeadDocument, error) {
	fullName := strings.TrimSpace(req.FullName)
	phone := digitsOnly(req.Phone)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	school := strings.TrimSpace(req.School)
	city := strings.TrimSpace(req.City)
	subjects := strings.TrimSpace(req.Subjects)
	classes := strings.TrimSpace(req.Classes)
	source := strings.TrimSpace(req.Source)

	switch {
	case fullName == "":
		return LeadDocument{}, errors.New("full name is required")
	case len(phone) != 10:
		return LeadDocument{}, errors.New("phone must contain exactly 10 digits")
	case email == "" || !strings.Contains(email, "@"):
		return LeadDocument{}, errors.New("valid email is required")
	case source == "":
		return LeadDocument{}, errors.New("source is required")
	}

	return LeadDocument{
		FullName:  fullName,
		Phone:     phone,
		Email:     email,
		School:    school,
		City:      city,
		Subjects:  subjects,
		Classes:   classes,
		Source:    source,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func digitsOnly(value string) string {
	var builder strings.Builder
	for _, ch := range value {
		if ch >= '0' && ch <= '9' {
			builder.WriteRune(ch)
		}
	}
	return builder.String()
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}
