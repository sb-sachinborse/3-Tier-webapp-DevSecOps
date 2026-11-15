package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

type App struct {
	db        *mongo.Database
	bucket    *gridfs.Bucket
	jwtSecret []byte
}

type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string             `bson:"name" json:"name"`
	Email     string             `bson:"email" json:"email"`
	Password  string             `bson:"password" json:"-"`
	Role      string             `bson:"role" json:"role"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

type Property struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Title       string             `bson:"title" json:"title"`
	Description string             `bson:"description" json:"description"`
	Price       float64            `bson:"price" json:"price"`
	Purpose     string             `bson:"purpose" json:"purpose"`
	Type        string             `bson:"type" json:"type"`
	Location    string             `bson:"location" json:"location"`
	Bedrooms    int                `bson:"bedrooms" json:"bedrooms"`
	Bathrooms   int                `bson:"bathrooms" json:"bathrooms"`
	Area        float64            `bson:"area" json:"area"`
	Features    []string           `bson:"features" json:"features"`
	ImageIDs    []string           `bson:"imageIds" json:"imageIds"`
	OwnerID     primitive.ObjectID `bson:"ownerId" json:"ownerId"`
	OwnerName   string             `bson:"ownerName" json:"ownerName"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type contextKey string

const userContextKey contextKey = "user"

func main() {
	uri := env("MONGO_URI", "mongodb://localhost:27017")
	dbName := env("MONGO_DATABASE", "dubai_properties")
	secret := env("JWT_SECRET", "")
	if len(secret) < 32 {
		log.Fatal("JWT_SECRET must contain at least 32 characters")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatal(err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal(err)
	}
	db := client.Database(dbName)
	bucket, err := gridfs.NewBucket(db)
	if err != nil {
		log.Fatal(err)
	}
	app := &App{db: db, bucket: bucket, jwtSecret: []byte(secret)}
	app.ensureIndexes(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", app.health)
	mux.HandleFunc("POST /api/auth/register", app.register)
	mux.HandleFunc("POST /api/auth/login", app.login)
	mux.Handle("GET /api/auth/me", app.auth(http.HandlerFunc(app.me)))
	mux.HandleFunc("GET /api/properties", app.listProperties)
	mux.HandleFunc("GET /api/properties/{id}", app.getProperty)
	mux.Handle("POST /api/properties", app.auth(http.HandlerFunc(app.createProperty)))
	mux.Handle("PUT /api/properties/{id}", app.auth(http.HandlerFunc(app.updateProperty)))
	mux.Handle("DELETE /api/properties/{id}", app.auth(http.HandlerFunc(app.deleteProperty)))
	mux.HandleFunc("GET /api/images/{id}", app.getImage)

	handler := cors(mux)
	port := env("PORT", "8081")
	log.Printf("Dubai Properties API listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
func (a *App) ensureIndexes(ctx context.Context) {
	_, err := a.db.Collection("users").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true),
	})
	if err != nil {
		log.Printf("user index: %v", err)
	}
	_, _ = a.db.Collection("properties").Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "createdAt", Value: -1}}})
}
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", env("CORS_ORIGIN", "http://localhost:5173"))
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func errorJSON(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func decodeJSON(r *http.Request, dst any) error {
	d := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	d.DisallowUnknownFields()
	return d.Decode(dst)
}
func (a *App) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "UP", "service": "dubai-properties-api"})
}

func validEmail(s string) bool {
	return strings.Contains(s, "@") && strings.Contains(strings.Split(s, "@")[1], ".")
}
func (a *App) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if decodeJSON(r, &in) != nil {
		errorJSON(w, 400, "Invalid request body")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if len(in.Name) < 2 || !validEmail(in.Email) || len(in.Password) < 8 {
		errorJSON(w, 400, "Name, valid email and password (8+ characters) are required")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		errorJSON(w, 500, "Could not secure password")
		return
	}
	u := User{Name: in.Name, Email: in.Email, Password: string(hash), Role: "user", CreatedAt: time.Now().UTC()}
	res, err := a.db.Collection("users").InsertOne(r.Context(), u)
	if mongo.IsDuplicateKeyError(err) {
		errorJSON(w, 409, "An account with this email already exists")
		return
	}
	if err != nil {
		errorJSON(w, 500, "Could not create account")
		return
	}
	u.ID = res.InsertedID.(primitive.ObjectID)
	token, err := a.token(u)
	if err != nil {
		errorJSON(w, 500, "Could not create session")
		return
	}
	writeJSON(w, 201, map[string]any{"token": token, "user": u})
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if decodeJSON(r, &in) != nil {
		errorJSON(w, 400, "Invalid request body")
		return
	}
	var u User
	err := a.db.Collection("users").FindOne(r.Context(), bson.M{"email": strings.ToLower(strings.TrimSpace(in.Email))}).Decode(&u)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(in.Password)) != nil {
		errorJSON(w, 401, "Invalid email or password")
		return
	}
	token, err := a.token(u)
	if err != nil {
		errorJSON(w, 500, "Could not create session")
		return
	}
	writeJSON(w, 200, map[string]any{"token": token, "user": u})
}
func (a *App) token(u User) (string, error) {
	ttl, _ := strconv.Atoi(env("JWT_TTL_HOURS", "24"))
	claims := jwt.MapClaims{"sub": u.ID.Hex(), "email": u.Email, "role": u.Role, "exp": time.Now().Add(time.Duration(ttl) * time.Hour).Unix(), "iat": time.Now().Unix()}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.jwtSecret)
}
func (a *App) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if h == "" || h == r.Header.Get("Authorization") {
			errorJSON(w, 401, "Authentication required")
			return
		}
		t, err := jwt.Parse(h, func(t *jwt.Token) (any, error) {
			if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, errors.New("unexpected signing method")
			}
			return a.jwtSecret, nil
		})
		if err != nil || !t.Valid {
			errorJSON(w, 401, "Invalid or expired session")
			return
		}
		c, ok := t.Claims.(jwt.MapClaims)
		if !ok {
			errorJSON(w, 401, "Invalid session")
			return
		}
		id, err := primitive.ObjectIDFromHex(fmt.Sprint(c["sub"]))
		if err != nil {
			errorJSON(w, 401, "Invalid session")
			return
		}
		var u User
		if a.db.Collection("users").FindOne(r.Context(), bson.M{"_id": id}).Decode(&u) != nil {
			errorJSON(w, 401, "Account not found")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, u)))
	})
}
func currentUser(r *http.Request) User { u, _ := r.Context().Value(userContextKey).(User); return u }
func (a *App) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"user": currentUser(r)})
}

func (a *App) listProperties(w http.ResponseWriter, r *http.Request) {
	q := bson.M{}
	s := strings.TrimSpace(r.URL.Query().Get("q"))
	if s != "" {
		q["$or"] = bson.A{bson.M{"title": bson.M{"$regex": regexp.QuoteMeta(s), "$options": "i"}}, bson.M{"location": bson.M{"$regex": regexp.QuoteMeta(s), "$options": "i"}}}
	}
	if v := r.URL.Query().Get("purpose"); v == "sale" || v == "rent" {
		q["purpose"] = v
	}
	if v := r.URL.Query().Get("type"); v != "" {
		q["type"] = v
	}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(100)
	cur, err := a.db.Collection("properties").Find(r.Context(), q, opts)
	if err != nil {
		errorJSON(w, 500, "Could not load properties")
		return
	}
	defer cur.Close(r.Context())
	items := []Property{}
	if err := cur.All(r.Context(), &items); err != nil {
		errorJSON(w, 500, "Could not decode properties")
		return
	}
	writeJSON(w, 200, map[string]any{"properties": items, "count": len(items)})
}
func parseID(w http.ResponseWriter, r *http.Request) (primitive.ObjectID, bool) {
	id, err := primitive.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		errorJSON(w, 400, "Invalid property ID")
		return primitive.NilObjectID, false
	}
	return id, true
}
func (a *App) getProperty(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var p Property
	if a.db.Collection("properties").FindOne(r.Context(), bson.M{"_id": id}).Decode(&p) != nil {
		errorJSON(w, 404, "Property not found")
		return
	}
	writeJSON(w, 200, map[string]any{"property": p})
}
func formProperty(r *http.Request, owner User) (Property, error) {
	price, e := strconv.ParseFloat(r.FormValue("price"), 64)
	if e != nil || price <= 0 {
		return Property{}, errors.New("price must be greater than zero")
	}
	beds, _ := strconv.Atoi(r.FormValue("bedrooms"))
	baths, _ := strconv.Atoi(r.FormValue("bathrooms"))
	area, _ := strconv.ParseFloat(r.FormValue("area"), 64)
	purpose := r.FormValue("purpose")
	if purpose != "sale" && purpose != "rent" {
		return Property{}, errors.New("purpose must be sale or rent")
	}
	title := strings.TrimSpace(r.FormValue("title"))
	loc := strings.TrimSpace(r.FormValue("location"))
	if len(title) < 5 || len(loc) < 2 || len(r.FormValue("description")) < 20 {
		return Property{}, errors.New("title, location and description are required")
	}
	features := []string{}
	for _, f := range strings.Split(r.FormValue("features"), ",") {
		if strings.TrimSpace(f) != "" {
			features = append(features, strings.TrimSpace(f))
		}
	}
	return Property{Title: title, Description: r.FormValue("description"), Price: price, Purpose: purpose, Type: r.FormValue("type"), Location: loc, Bedrooms: beds, Bathrooms: baths, Area: area, Features: features, OwnerID: owner.ID, OwnerName: owner.Name}, nil
}
func (a *App) saveImages(ctx context.Context, files []*multipart.FileHeader) ([]string, error) {
	ids := []string{}
	for _, fh := range files {
		if fh.Size > 8<<20 {
			return ids, fmt.Errorf("%s exceeds 8MB", fh.Filename)
		}
		f, err := fh.Open()
		if err != nil {
			return ids, err
		}
		ct := fh.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "image/") {
			f.Close()
			return ids, errors.New("only image files are accepted")
		}
		id, err := a.bucket.UploadFromStream(fh.Filename, f, options.GridFSUpload().SetMetadata(bson.M{"contentType": ct}))
		f.Close()
		if err != nil {
			return ids, err
		}
		ids = append(ids, id.Hex())
	}
	return ids, nil
}
func (a *App) createProperty(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 40<<20)
	if err := r.ParseMultipartForm(40 << 20); err != nil {
		errorJSON(w, 400, "Invalid multipart form or upload too large")
		return
	}
	p, err := formProperty(r, currentUser(r))
	if err != nil {
		errorJSON(w, 400, err.Error())
		return
	}
	if r.MultipartForm != nil {
		p.ImageIDs, err = a.saveImages(r.Context(), r.MultipartForm.File["images"])
		if err != nil {
			errorJSON(w, 400, err.Error())
			return
		}
	}
	p.CreatedAt = time.Now().UTC()
	p.UpdatedAt = p.CreatedAt
	res, err := a.db.Collection("properties").InsertOne(r.Context(), p)
	if err != nil {
		errorJSON(w, 500, "Could not save property")
		return
	}
	p.ID = res.InsertedID.(primitive.ObjectID)
	writeJSON(w, 201, map[string]any{"property": p})
}
func (a *App) updateProperty(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var old Property
	if a.db.Collection("properties").FindOne(r.Context(), bson.M{"_id": id}).Decode(&old) != nil {
		errorJSON(w, 404, "Property not found")
		return
	}
	u := currentUser(r)
	if u.Role != "admin" && old.OwnerID != u.ID {
		errorJSON(w, 403, "You can only edit your own listings")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 40<<20)
	if err := r.ParseMultipartForm(40 << 20); err != nil {
		errorJSON(w, 400, "Invalid multipart form")
		return
	}
	p, err := formProperty(r, u)
	if err != nil {
		errorJSON(w, 400, err.Error())
		return
	}
	p.ID = id
	p.OwnerID = old.OwnerID
	p.OwnerName = old.OwnerName
	p.CreatedAt = old.CreatedAt
	p.UpdatedAt = time.Now().UTC()
	p.ImageIDs = old.ImageIDs
	if r.MultipartForm != nil && len(r.MultipartForm.File["images"]) > 0 {
		ids, e := a.saveImages(r.Context(), r.MultipartForm.File["images"])
		if e != nil {
			errorJSON(w, 400, e.Error())
			return
		}
		p.ImageIDs = append(p.ImageIDs, ids...)
	}
	_, err = a.db.Collection("properties").ReplaceOne(r.Context(), bson.M{"_id": id}, p)
	if err != nil {
		errorJSON(w, 500, "Could not update property")
		return
	}
	writeJSON(w, 200, map[string]any{"property": p})
}
func (a *App) deleteProperty(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var p Property
	if a.db.Collection("properties").FindOne(r.Context(), bson.M{"_id": id}).Decode(&p) != nil {
		errorJSON(w, 404, "Property not found")
		return
	}
	u := currentUser(r)
	if u.Role != "admin" && p.OwnerID != u.ID {
		errorJSON(w, 403, "You can only delete your own listings")
		return
	}
	_, err := a.db.Collection("properties").DeleteOne(r.Context(), bson.M{"_id": id})
	if err != nil {
		errorJSON(w, 500, "Could not delete property")
		return
	}
	for _, s := range p.ImageIDs {
		if oid, e := primitive.ObjectIDFromHex(s); e == nil {
			_ = a.bucket.Delete(oid)
		}
	}
	writeJSON(w, 200, map[string]string{"message": "Property deleted"})
}
func (a *App) getImage(w http.ResponseWriter, r *http.Request) {
	id, err := primitive.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var meta struct {
		Metadata bson.M `bson:"metadata"`
	}
	if a.db.Collection("fs.files").FindOne(r.Context(), bson.M{"_id": id}).Decode(&meta) != nil {
		http.NotFound(w, r)
		return
	}
	ct := "image/jpeg"
	if v, ok := meta.Metadata["contentType"].(string); ok && strings.HasPrefix(v, "image/") {
		ct = v
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	if _, err := a.bucket.DownloadToStream(id, w); err != nil {
		log.Printf("image stream: %v", err)
	}
}
