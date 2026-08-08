package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/leoldding/wl-leaderboard/internal/awsclients"
)

const requestTimeout = 5 * time.Second

var (
	errInvalidJSON    = errors.New("invalid json body")
	errIDRequired     = errors.New("id is required")
	errNameRequired   = errors.New("name is required")
	errSnatchPositive = errors.New("snatch must be a positive number")
	errCleanJerkPos   = errors.New("cleanjerk must be a positive number")
	errNotFound       = errors.New("row not found")
)

type LeaderboardHandler struct {
	clients   *awsclients.Clients
	tableName string
}

type leaderboardRow struct {
	ID        string `json:"id" dynamodbav:"id"`
	Name      string `json:"name" dynamodbav:"name"`
	Snatch    int    `json:"snatch" dynamodbav:"snatch"`
	CleanJerk int    `json:"cleanjerk" dynamodbav:"cleanjerk"`
	Total     int    `json:"total" dynamodbav:"total"`
}

type rowPayload struct {
	Name      string `json:"name"`
	Snatch    int    `json:"snatch"`
	CleanJerk int    `json:"cleanjerk"`
}

func newLeaderboardRow(id, name string, snatch, cleanjerk int) leaderboardRow {
	return leaderboardRow{
		ID:        id,
		Name:      name,
		Snatch:    snatch,
		CleanJerk: cleanjerk,
		Total:     snatch + cleanjerk,
	}
}

func NewLeaderboardHandler(clients *awsclients.Clients, tableName string) *LeaderboardHandler {
	return &LeaderboardHandler{
		clients:   clients,
		tableName: tableName,
	}
}

func (l *LeaderboardHandler) GetRows(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	var rows []map[string]types.AttributeValue
	input := &dynamodb.ScanInput{
		TableName: aws.String(l.tableName),
	}

	for {
		resp, err := l.clients.DynamoDB.Scan(ctx, input)
		if err != nil {
			log.Printf("dynamodb scan error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		rows = append(rows, resp.Items...)

		if resp.LastEvaluatedKey == nil {
			break
		}

		input.ExclusiveStartKey = resp.LastEvaluatedKey
	}

	var leaderboardRows []leaderboardRow

	err := attributevalue.UnmarshalListOfMaps(rows, &leaderboardRows)
	if err != nil {
		log.Printf("list unmarshal error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	sort.SliceStable(leaderboardRows, func(i, j int) bool {
		if leaderboardRows[i].Total != leaderboardRows[j].Total {
			return leaderboardRows[i].Total > leaderboardRows[j].Total
		}
		return leaderboardRows[i].Name < leaderboardRows[j].Name
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err = json.NewEncoder(w).Encode(leaderboardRows); err != nil {
		log.Printf("encode error: %v", err)
	}
}

func (l *LeaderboardHandler) AddRow(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	var payload rowPayload

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, errInvalidJSON.Error(), http.StatusBadRequest)
		return
	}

	if err := validateRow(payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	row := newLeaderboardRow(uuid.NewString(), payload.Name, payload.Snatch, payload.CleanJerk)

	item, err := attributevalue.MarshalMap(row)
	if err != nil {
		log.Printf("marshal error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	_, err = l.clients.DynamoDB.PutItem(ctx, &dynamodb.PutItemInput{
		Item:      item,
		TableName: aws.String(l.tableName),
	})
	if err != nil {
		log.Printf("dynamodb put item error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err = json.NewEncoder(w).Encode(row); err != nil {
		log.Printf("error encoding response: %v", err)
	}
}

func (l *LeaderboardHandler) UpdateRow(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, errIDRequired.Error(), http.StatusBadRequest)
		return
	}

	var payload rowPayload

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, errInvalidJSON.Error(), http.StatusBadRequest)
		return
	}

	if err := validateRow(payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	row := newLeaderboardRow(id, payload.Name, payload.Snatch, payload.CleanJerk)

	key, err := keyOf(id)
	if err != nil {
		log.Printf("marshal key error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	_, err = l.clients.DynamoDB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:           aws.String(l.tableName),
		Key:                 key,
		UpdateExpression:    aws.String("SET #n = :name, snatch = :snatch, cleanjerk = :cleanjerk, #t = :total"),
		ConditionExpression: aws.String("attribute_exists(id)"),
		ExpressionAttributeNames: map[string]string{
			"#n": "name",
			"#t": "total",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":name":      &types.AttributeValueMemberS{Value: row.Name},
			":snatch":    &types.AttributeValueMemberN{Value: strconv.Itoa(row.Snatch)},
			":cleanjerk": &types.AttributeValueMemberN{Value: strconv.Itoa(row.CleanJerk)},
			":total":     &types.AttributeValueMemberN{Value: strconv.Itoa(row.Total)},
		},
	})
	if err != nil {
		var condErr *types.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			http.Error(w, errNotFound.Error(), http.StatusNotFound)
			return
		}
		log.Printf("dynamodb update item error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err = json.NewEncoder(w).Encode(row); err != nil {
		log.Printf("error encoding response: %v", err)
	}
}

func (l *LeaderboardHandler) DeleteRow(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, errIDRequired.Error(), http.StatusBadRequest)
		return
	}

	key, err := keyOf(id)
	if err != nil {
		log.Printf("marshal key error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	_, err = l.clients.DynamoDB.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		Key:                 key,
		TableName:           aws.String(l.tableName),
		ConditionExpression: aws.String("attribute_exists(id)"),
	})
	if err != nil {
		var condErr *types.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			http.Error(w, errNotFound.Error(), http.StatusNotFound)
			return
		}
		log.Printf("dynamodb delete item error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func keyOf(id string) (map[string]types.AttributeValue, error) {
	return attributevalue.MarshalMap(map[string]string{"id": id})
}

func validateRow(r rowPayload) error {
	if strings.TrimSpace(r.Name) == "" {
		return errNameRequired
	}
	if r.Snatch <= 0 {
		return errSnatchPositive
	}
	if r.CleanJerk <= 0 {
		return errCleanJerkPos
	}
	return nil
}
