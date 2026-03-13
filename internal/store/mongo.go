package store

import (
	"context"
	"fmt"
	"time"

	"github.com/gustycube/spyder/internal/emit"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"
)

// MongoStore persists batches to MongoDB with separate collections
// for domains, IPs, certs, and edges.
type MongoStore struct {
	client  *mongo.Client
	db      *mongo.Database
	domains *mongo.Collection
	ips     *mongo.Collection
	certs   *mongo.Collection
	edges   *mongo.Collection
	log     *zap.SugaredLogger
}

// NewMongo connects to MongoDB and sets up collections with indexes.
func NewMongo(ctx context.Context, uri, dbName string, log *zap.SugaredLogger) (*MongoStore, error) {
	if dbName == "" {
		dbName = "spyder"
	}

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}

	// Verify connection
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}

	db := client.Database(dbName)
	s := &MongoStore{
		client:  client,
		db:      db,
		domains: db.Collection("domains"),
		ips:     db.Collection("ips"),
		certs:   db.Collection("certs"),
		edges:   db.Collection("edges"),
		log:     log,
	}

	if err := s.ensureIndexes(ctx); err != nil {
		log.Warnw("mongo index creation", "err", err)
	}

	return s, nil
}

func (s *MongoStore) ensureIndexes(ctx context.Context) error {
	idxCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Domains: unique on host
	_, err := s.domains.Indexes().CreateOne(idxCtx, mongo.IndexModel{
		Keys:    bson.D{{Key: "host", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	// IPs: unique on ip
	_, err = s.ips.Indexes().CreateOne(idxCtx, mongo.IndexModel{
		Keys:    bson.D{{Key: "ip", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	// Certs: unique on spki_sha256
	_, err = s.certs.Indexes().CreateOne(idxCtx, mongo.IndexModel{
		Keys:    bson.D{{Key: "spki_sha256", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	// Edges: compound index on type+source+target, index on source, index on target
	_, err = s.edges.Indexes().CreateMany(idxCtx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "type", Value: 1}, {Key: "source", Value: 1}, {Key: "target", Value: 1}}},
		{Keys: bson.D{{Key: "source", Value: 1}}},
		{Keys: bson.D{{Key: "target", Value: 1}}},
	})
	return err
}

// WriteBatch upserts all nodes and inserts edges from a batch.
func (s *MongoStore) WriteBatch(ctx context.Context, batch emit.Batch) error {
	// Upsert domains
	if len(batch.NodesD) > 0 {
		models := make([]mongo.WriteModel, len(batch.NodesD))
		for i, d := range batch.NodesD {
			models[i] = mongo.NewUpdateOneModel().
				SetFilter(bson.M{"host": d.Host}).
				SetUpdate(bson.M{
					"$set": bson.M{
						"host":      d.Host,
						"apex":      d.Apex,
						"last_seen": d.LastSeen,
					},
					"$setOnInsert": bson.M{
						"first_seen": d.FirstSeen,
					},
				}).
				SetUpsert(true)
		}
		if _, err := s.domains.BulkWrite(ctx, models); err != nil {
			s.log.Warnw("mongo upsert domains", "err", err, "count", len(batch.NodesD))
		}
	}

	// Upsert IPs
	if len(batch.NodesIP) > 0 {
		models := make([]mongo.WriteModel, len(batch.NodesIP))
		for i, ip := range batch.NodesIP {
			models[i] = mongo.NewUpdateOneModel().
				SetFilter(bson.M{"ip": ip.IP}).
				SetUpdate(bson.M{
					"$set": bson.M{
						"ip":        ip.IP,
						"last_seen": ip.LastSeen,
					},
					"$setOnInsert": bson.M{
						"first_seen": ip.FirstSeen,
					},
				}).
				SetUpsert(true)
		}
		if _, err := s.ips.BulkWrite(ctx, models); err != nil {
			s.log.Warnw("mongo upsert ips", "err", err, "count", len(batch.NodesIP))
		}
	}

	// Upsert certs
	if len(batch.NodesC) > 0 {
		models := make([]mongo.WriteModel, len(batch.NodesC))
		for i, c := range batch.NodesC {
			models[i] = mongo.NewUpdateOneModel().
				SetFilter(bson.M{"spki_sha256": c.SPKI}).
				SetUpdate(bson.M{
					"$set": bson.M{
						"spki_sha256": c.SPKI,
						"subject_cn":  c.SubjectCN,
						"issuer_cn":   c.IssuerCN,
						"not_before":  c.NotBefore,
						"not_after":   c.NotAfter,
					},
				}).
				SetUpsert(true)
		}
		if _, err := s.certs.BulkWrite(ctx, models); err != nil {
			s.log.Warnw("mongo upsert certs", "err", err, "count", len(batch.NodesC))
		}
	}

	// Insert edges (duplicates are fine — they represent repeated observations)
	if len(batch.Edges) > 0 {
		docs := make([]interface{}, len(batch.Edges))
		for i, e := range batch.Edges {
			docs[i] = bson.M{
				"type":        e.Type,
				"source":      e.Source,
				"target":      e.Target,
				"observed_at": e.ObservedAt,
				"probe_id":    e.ProbeID,
				"run_id":      e.RunID,
			}
		}
		if _, err := s.edges.InsertMany(ctx, docs); err != nil {
			s.log.Warnw("mongo insert edges", "err", err, "count", len(batch.Edges))
		}
	}

	return nil
}

// Close disconnects from MongoDB.
func (s *MongoStore) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}
