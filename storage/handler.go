package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	protov1 "go.bryk.io/covid-tracking/proto/v1"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Handler provides the main interface to abstract away storage
// operations.
type Handler struct {
	cl *mongo.Client
	db *mongo.Database
}

const (
	database     string = "ct19"       // Database name
	userCodeTTL  int32  = 60           // User activation codes expire after 1 minute
	agentCodeTTL int32  = 60 * 60 * 24 // Agent activation codes expire after a day
)

// GeoJSON structure for location records.
type location struct {
	Type        string     `json:"type"`
	Coordinates [2]float32 `json:"coordinates"`
}

// NewHandler returns a new storage handler.
func NewHandler(sink string) (*Handler, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if !strings.HasPrefix(sink, "mongodb://") {
		sink = fmt.Sprintf("mongodb://%s", sink)
	}

	// Open connection
	cl, err := mongo.Connect(ctx, options.Client().ApplyURI(sink))
	if err != nil {
		return nil, err
	}

	// Ensure server is reachable
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := cl.Ping(ctx, readpref.Primary()); err != nil {
		return nil, errors.Wrap(err, "failed to contact server")
	}

	// Setup handle instance
	st := &Handler{
		cl: cl,
		db: cl.Database(database),
	}
	if err := st.setup(); err != nil {
		return nil, err
	}
	return st, nil
}

// Close the handler instance.
func (st *Handler) Close() {
	_ = st.cl.Disconnect(context.Background())
}

// ActivationCode creates a new activation code. The code will expire automatically.
func (st *Handler) ActivationCode(req *protov1.ActivationCodeRequest) (string, error) {
	ac := uuid.New()
	record := bson.M{
		"did":     req.Did,
		"code":    ac.String(),
		"created": time.Now(),
	}
	ctx, cancel := context.WithTimeout(context.TODO(), 2*time.Second)
	defer cancel()
	_, err := st.db.Collection(fmt.Sprintf("%s_codes", req.Role)).InsertOne(ctx, record)
	return ac.String(), err
}

// VerifyActivationCode checks if the provided registration token is valid.
// If the token is valid it will be deleted automatically.
func (st *Handler) VerifyActivationCode(req *protov1.CredentialsRequest) bool {
	query := bson.M{
		"did":  req.Did,
		"code": req.ActivationCode,
	}
	col := st.db.Collection(fmt.Sprintf("%s_codes", req.Role))
	ctx, cancel := context.WithTimeout(context.TODO(), 2*time.Second)
	defer cancel()
	res := col.FindOne(ctx, query)
	valid := res.Err() == nil
	if valid {
		_, _ = col.DeleteMany(ctx, query)
	}
	return valid
}

// LocationRecords add and index location entries to persistent storage.
func (st *Handler) LocationRecords(records []*protov1.LocationRecord) error {
	// Prepare entries
	entries := make([]interface{}, len(records))
	for i, r := range records {
		entries[i] = bson.M{
			"did":       r.Did,
			"timestamp": time.Unix(r.Timestamp, 0),
			"hash":      r.Hash,
			"proof":     r.Proof,
			"location":  getLocation(r),
		}
	}

	// Save records
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()
	_, err := st.db.Collection("records").InsertMany(ctx, entries)
	return err
}

func (st *Handler) setup() error {
	// TTL user codes
	userCodes := st.db.Collection("user_codes")
	if _, err := userCodes.Indexes().CreateOne(context.Background(), ttlIndex(userCodeTTL)); err != nil {
		return err
	}

	// TTL agent codes
	agentCodes := st.db.Collection("agent_codes")
	if _, err := agentCodes.Indexes().CreateOne(context.Background(), ttlIndex(agentCodeTTL)); err != nil {
		return err
	}

	// GeoSpatial and timestamp indexes on record.location
	records := st.db.Collection("records")
	if _, err := records.Indexes().CreateOne(context.Background(), geoIndex("location")); err != nil {
		return err
	}
	_, err := records.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.M{
			"timestamp": 1,
		},
	})
	if err != nil {
		return err
	}
	return nil
}

func ttlIndex(ttl int32) mongo.IndexModel {
	return mongo.IndexModel{
		Keys: bson.M{
			"created": 1,
		},
		Options: &options.IndexOptions{
			ExpireAfterSeconds: &ttl,
		},
	}
}

func geoIndex(field string) mongo.IndexModel {
	return mongo.IndexModel{
		Keys: bson.M{
			field: "2dsphere",
		},
	}
}

func getLocation(r *protov1.LocationRecord) *location {
	return &location{
		Type: "Point",
		Coordinates: [2]float32{
			r.Lng,
			r.Lat,
		},
	}
}
