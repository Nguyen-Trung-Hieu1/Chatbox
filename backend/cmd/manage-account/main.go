package main

import (
	"backend/internal/config"
	"context"
	"flag"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"time"
)

// Local operator command. Never exposed through the public HTTP API.
func main() {
	username := flag.String("username", "", "existing username")
	plan := flag.String("plan", "", "free or paid; omit to leave unchanged")
	add := flag.Int64("add-credits", 0, "credits to add")
	flag.Parse()
	if *username == "" || (*plan != "" && *plan != "free" && *plan != "paid") || *add < 0 || (*plan == "" && *add == 0) {
		log.Fatal("use -username NAME [-plan free|paid] [-add-credits N]")
	}
	cfg, err := config.Load(".env", "backend/.env")
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	db := client.Database(cfg.MongoDatabase)
	var user struct {
		ID any `bson:"_id"`
	}
	if err = db.Collection("users").FindOne(ctx, bson.M{"username": *username}).Decode(&user); err != nil {
		log.Fatal(err)
	}
	_, err = db.Collection("wallets").UpdateOne(ctx, bson.M{"_id": user.ID}, bson.M{"$setOnInsert": bson.M{"plan": "free", "balance": cfg.FreeCredits, "usage": bson.A{}}}, options.Update().SetUpsert(true))
	if err != nil {
		log.Fatal(err)
	}
	update := bson.M{}
	if *plan != "" {
		update["$set"] = bson.M{"plan": *plan}
	}
	if *add > 0 {
		update["$inc"] = bson.M{"balance": *add}
	}
	if _, err = db.Collection("wallets").UpdateOne(ctx, bson.M{"_id": user.ID}, update); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Updated %s\n", *username)
}
