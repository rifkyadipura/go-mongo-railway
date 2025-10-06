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

// collection adalah variabel global untuk menyimpan koneksi ke koleksi MongoDB
var collection *mongo.Collection

// ctx adalah variabel global untuk context, digunakan di semua operasi database
var ctx = context.Background()

// Point mendefinisikan struktur GeoJSON Point sesuai standar MongoDB
type Point struct {
	Type        string    `bson:"type" json:"type"`
	Coordinates []float64 `bson:"coordinates" json:"coordinates"`
}

// Location adalah model data (struct) untuk setiap lokasi yang disimpan
type Location struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Name        string             `bson:"name" json:"name"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	Location    Point              `bson:"location" json:"location"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
}

// initDB berfungsi untuk menginisialisasi koneksi ke database MongoDB
func initDB() {
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found, reading environment variables from system")
	}

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

	collection = client.Database("test").Collection("locations")

	indexModel := mongo.IndexModel{
		Keys: bson.M{"location": "2dsphere"},
	}
	_, err = collection.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		fmt.Printf("Index creation might have failed (or already exists): %v\n", err)
	} else {
		fmt.Println("2dsphere index on 'location' field verified.")
	}
}

// createLocationHandler: Saat sukses, mengembalikan data yang baru dibuat. Ini sudah pesan sukses yang sangat baik.
func createLocationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
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

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(loc)
}

// getLocationsHandler: Saat sukses, mengembalikan array data. Ini juga sudah merupakan pesan sukses.
func getLocationsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
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

	json.NewEncoder(w).Encode(locations)
}

// updateLocationHandler menangani request PUT untuk memperbarui data lokasi
func updateLocationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid location ID format", http.StatusBadRequest)
		return
	}

	var loc Location
	if err := json.NewDecoder(r.Body).Decode(&loc); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

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

	// --- PERUBAHAN DI SINI ---
	// Mengirimkan pesan sukses dalam format JSON yang terstruktur
	w.WriteHeader(http.StatusOK)
	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Location with ID %s was successfully updated", vars["id"]),
	}
	json.NewEncoder(w).Encode(response)
}

// deleteLocationHandler menangani request DELETE untuk menghapus data lokasi
func deleteLocationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid location ID format", http.StatusBadRequest)
		return
	}

	result, err := collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if result.DeletedCount == 0 {
		http.Error(w, "Location not found", http.StatusNotFound)
		return
	}

	// --- PERUBAHAN DI SINI ---
	// Mengganti 204 No Content menjadi 200 OK agar bisa mengirim pesan
	w.WriteHeader(http.StatusOK)
	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Location with ID %s was successfully deleted", vars["id"]),
	}
	json.NewEncoder(w).Encode(response)
}

// main adalah fungsi utama tempat aplikasi dimulai
func main() {
	initDB()

	r := mux.NewRouter()

	r.HandleFunc("/locations", createLocationHandler).Methods("POST")
	r.HandleFunc("/locations", getLocationsHandler).Methods("GET")
	r.HandleFunc("/locations/{id}", updateLocationHandler).Methods("PUT")
	r.HandleFunc("/locations/{id}", deleteLocationHandler).Methods("DELETE")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Server starting on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
