package store

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const dbName = "sundial"

type MongoStore struct {
	client  *mongo.Client
	db      *mongo.Database
	records *mongo.Collection
}

func NewMongoStore(url string) (*MongoStore, error) {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(url))
	if err != nil {
		return nil, err
	}

	db := client.Database(dbName)
	tasks := db.Collection("tasks")
	records := db.Collection("records")

	// Create indexes for tasks collection
	tasksIndexModel := []mongo.IndexModel{
		{
			Keys:    bson.M{"key": 1},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.M{"next_run_at": 1},
		},
	}

	_, err = tasks.Indexes().CreateMany(context.Background(), tasksIndexModel)
	if err != nil {
		return nil, err
	}

	// Create indexes for records collection
	recordsIndexModel := []mongo.IndexModel{
		{
			Keys:    bson.M{"key": 1},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.M{"task_key": 1},
		},
	}

	_, err = records.Indexes().CreateMany(context.Background(), recordsIndexModel)
	if err != nil {
		return nil, err
	}

	return &MongoStore{
		client:  client,
		db:      db,
		records: records,
	}, nil
}

func (s *MongoStore) Clear() error {

	if err := s.records.Drop(context.Background()); err != nil {
		return err
	}

	return nil
}

func (s *MongoStore) Close() error {
	return s.client.Disconnect(context.Background())
}

func (s *MongoStore) GetString(key string) (string, error) {
	var result struct {
		Value string `bson:"value"`
	}

	err := s.records.FindOne(context.Background(), bson.M{"key": key}).Decode(&result)
	if err != nil {
		return "", err
	}

	return result.Value, nil
}

func (s *MongoStore) SetString(key, value string) error {
	_, err := s.records.UpdateOne(context.Background(), bson.M{"key": key},
		bson.M{"$set": bson.M{"value": value}}, options.Update().SetUpsert(true))
	return err
}
