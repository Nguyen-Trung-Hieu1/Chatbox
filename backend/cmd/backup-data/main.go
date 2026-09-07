// Export the configured MongoDB database as BSON files and index metadata.
// The destination must not exist, so a previous backup cannot be overwritten.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backend/internal/config"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: backup-data NEW_DESTINATION")
	}
	cfg, err := config.Load(".env", "backend/.env")
	if err != nil {
		return err
	}
	dest := os.Args[1]
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		return fmt.Errorf("destination must not exist")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		return fmt.Errorf("MongoDB connection failed")
	}
	defer client.Disconnect(context.Background())
	db := client.Database(cfg.MongoDatabase)
	specs, err := db.ListCollectionSpecifications(ctx, bson.D{})
	if err != nil {
		return err
	}
	if len(specs) == 0 {
		return fmt.Errorf("source database has no collections; refusing empty backup")
	}
	if err := os.MkdirAll(dest, 0700); err != nil {
		return err
	}
	counts := map[string]int64{}
	for _, spec := range specs {
		if spec.Type != "collection" || strings.HasPrefix(spec.Name, "system.") {
			return fmt.Errorf("unsupported collection type: %s", spec.Name)
		}
		if strings.ContainsAny(spec.Name, "/\\") {
			return fmt.Errorf("unsafe collection name")
		}
		col := db.Collection(spec.Name)
		cursor, err := col.Find(ctx, bson.D{})
		if err != nil {
			return err
		}
		f, err := os.OpenFile(filepath.Join(dest, spec.Name+".bson"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			cursor.Close(ctx)
			return err
		}
		var count int64
		for cursor.Next(ctx) {
			if _, err := f.Write(cursor.Current); err != nil {
				f.Close()
				cursor.Close(ctx)
				return err
			}
			count++
		}
		readErr := cursor.Err()
		cursor.Close(ctx)
		closeErr := f.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		idx, err := col.Indexes().List(ctx)
		if err != nil {
			return err
		}
		indexes := []bson.Raw{}
		if err := idx.All(ctx, &indexes); err != nil {
			return err
		}
		meta := bson.M{"indexes": indexes, "collectionName": spec.Name, "options": bson.M{}}
		if len(spec.Options) > 0 {
			meta["options"] = spec.Options
		}
		data, err := bson.MarshalExtJSON(meta, true, false)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dest, spec.Name+".metadata.json"), data, 0600); err != nil {
			return err
		}
		counts[spec.Name] = count
	}
	summary, err := json.MarshalIndent(counts, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dest, "backup-counts.json"), summary, 0600); err != nil {
		return err
	}
	fmt.Printf("Backup complete: database=%s collections=%d\n%s\n", cfg.MongoDatabase, len(counts), summary)
	return nil
}
