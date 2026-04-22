package main

import (
	"context"
	"crypto/tls"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDBOrderRepo struct {
	db *mongo.Collection
}

func NewMongoDBOrderRepo(mongoURI string, mongoDB string, mongoCollection string, mongoUser string, mongoPassword string) (*MongoDBOrderRepo, error) {
	ctx := context.Background()

	var clientOptions *options.ClientOptions
	if mongoUser == "" && mongoPassword == "" {
		clientOptions = options.Client().ApplyURI(mongoURI)
	} else {
		clientOptions = options.Client().ApplyURI(mongoURI).
			SetAuth(options.Credential{
				AuthSource: mongoDB,
				Username:   mongoUser,
				Password:   mongoPassword,
			}).
			SetTLSConfig(&tls.Config{InsecureSkipVerify: false})
	}

	mongoClient, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Printf("failed to connect to mongodb: %s", err)
		return nil, err
	}

	err = mongoClient.Ping(ctx, nil)
	if err != nil {
		log.Printf("failed to ping database: %s", err)
		return nil, err
	}

	log.Printf("pong from database")

	collection := mongoClient.Database(mongoDB).Collection(mongoCollection)
	return &MongoDBOrderRepo{db: collection}, nil
}

func (r *MongoDBOrderRepo) GetPendingOrders() ([]Order, error) {
	ctx := context.TODO()

	var orders []Order
	cursor, err := r.db.Find(ctx, bson.M{"status": Pending})
	if err != nil {
		log.Printf("Failed to find pending orders: %s", err)
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var pendingOrder Order
		if err := cursor.Decode(&pendingOrder); err != nil {
			log.Printf("Failed to decode order: %s", err)
			return nil, err
		}
		orders = append(orders, pendingOrder)
	}

	if err := cursor.Err(); err != nil {
		log.Printf("Cursor iteration error: %s", err)
		return nil, err
	}

	return orders, nil
}

func (r *MongoDBOrderRepo) GetOrder(id string) (Order, error) {
	ctx := context.TODO()

	filter := bson.M{"orderid": id}

	var order Order
	err := r.db.FindOne(ctx, filter).Decode(&order)
	if err != nil {
		log.Printf("Failed to decode order: %s", err)
		return order, err
	}

	return order, nil
}

func (r *MongoDBOrderRepo) UpdateOrder(order Order) error {
	ctx := context.TODO()

	filter := bson.M{"orderid": order.OrderID}
	update := bson.M{
		"$set": bson.M{
			"status": order.Status,
		},
	}

	log.Printf("Updating order: %v", order)
	updateResult, err := r.db.UpdateOne(ctx, filter, update)
	if err != nil {
		log.Printf("Failed to update order: %s", err)
		return err
	}

	log.Printf("Matched %v document and updated %v document.", updateResult.MatchedCount, updateResult.ModifiedCount)
	return nil
}