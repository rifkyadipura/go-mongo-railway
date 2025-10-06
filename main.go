package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var collection *mongo.Collection
var ctx = context.Background()

// Point mendefinisikan struktur GeoJSON untuk MongoDB
type Point struct {
	Type        string    `bson:"type"`
	Coordinates []float64 `bson:"coordinates"`
}

// Location adalah model data kita untuk MongoDB
type Location struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Name        string             `bson:"name"`
	Description string             `bson:"description,omitempty"`
	Location    Point              `bson:"location"`
	CreatedAt   time.Time          `bson:"created_at"`
}

func initDB() {
	// Coba load .env untuk development lokal
	godotenv.Load()

	mongoURL := os.Getenv("MONGO_PUBLIC_URL")
	if mongoURL == "" {
		log.Fatal("MONGO_PUBLIC_URL environment variable is not set")
	}

	clientOptions := options.Client().ApplyURI(mongoURL)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Successfully connected to MongoDB!")
	// Ganti "test" dengan nama database yang Anda inginkan
	// Ganti "locations" dengan nama collection yang Anda inginkan
	collection = client.Database("test").Collection("locations")

	// Membuat index geospasial agar bisa melakukan query lokasi
	// Ini SANGAT PENTING untuk performa query geospasial
	indexModel := mongo.IndexModel{
		Keys: bson.M{"location": "2dsphere"},
	}
	_, err = collection.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		// Abaikan jika index sudah ada, tapi log error lainnya
		fmt.Printf("Index creation might have failed (or already exists): %v\n", err)
	} else {
		fmt.Println("2dsphere index created successfully.")
	}
}

func createLocationHandler(w http.ResponseWriter, r *http.Request) {
	var loc Location
	if err := json.NewDecoder(r.Body).Decode(&loc); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	loc.ID = primitive.NewObjectID()
	loc.CreatedAt = time.Now()

	_, err := collection.InsertOne(ctx, loc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(loc)
}

func getLocationsHandler(w http.ResponseWriter, r *http.Request) {
	var locations []Location
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &locations); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(locations)
}

func updateLocationHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := primitive.ObjectIDFromHex(vars["id"])

	var loc Location
	if err := json.NewDecoder(r.Body).Decode(&loc); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Buat update filter
	update := bson.M{
		"$set": bson.M{
			"name":        loc.Name,
			"description": loc.Description,
			"location":    loc.Location,
		},
	}

	result, err := collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if result.MatchedCount == 0 {
		http.Error(w, "Location not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Location with ID %s updated", vars["id"])
}

func deleteLocationHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := primitive.ObjectIDFromHex(vars["id"])

	result, err := collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if result.DeletedCount == 0 {
		http.Error(w, "Location not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func main() {
	initDB()

	r := mux.NewRouter()
	r.HandleFunc("/locations", createLocationHandler).Methods("POST")
	r.HandleFunc("/locations", getLocationsHandler).Methods("GET")
	r.HandleFunc("/locations/{id}", updateLocationHandler).Methods("PUT")
	r.HandleFunc("/locations/{id}", deleteLocationHandler).Methods("DELETE")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Port default untuk local dev
	}

	fmt.Printf("Server starting on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
