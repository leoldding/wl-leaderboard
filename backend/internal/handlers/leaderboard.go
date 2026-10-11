package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"mime"
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

const (
	requestTimeout = 5 * time.Second
	scanTimeout    = 30 * time.Second
	maxBodyBytes   = 16 << 10
)

var (
	errInvalidJSON    = errors.New("invalid json body")
	errIDRequired     = errors.New("id is required")
	errNameRequired   = errors.New("name is required")
	errSnatchRange    = errors.New("snatch must be between 0 and 500 with at most 2 decimal places")
	errCleanJerkRange = errors.New("cleanjerk must be between 0 and 500 with at most 2 decimal places")
	errNotFound       = errors.New("row not found")
)

type LeaderboardHandler struct {
	clients   *awsclients.Clients
	tableName string
}

type leaderboardRow struct {
	ID        string  `json:"id" dynamodbav:"id"`
	Name      string  `json:"name" dynamodbav:"name"`
	Snatch    float64 `json:"snatch" dynamodbav:"snatch"`
	CleanJerk float64 `json:"cleanjerk" dynamodbav:"cleanjerk"`
	Total     float64 `json:"total" dynamodbav:"total"`
}

type rowPayload struct {
	Name      string  `json:"name"`
	Snatch    float64 `json:"snatch"`
	CleanJerk float64 `json:"cleanjerk"`
}

func newLeaderboardRow(id, name string, snatch, cleanjerk float64) leaderboardRow {
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
	ctx, cancel := context.WithTimeout(r.Context(), scanTimeout)
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

	if err := decodeRowPayload(w, r, &payload); err != nil {
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

	if err := decodeRowPayload(w, r, &payload); err != nil {
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
			":snatch":    &types.AttributeValueMemberN{Value: strconv.FormatFloat(row.Snatch, 'f', -1, 64)},
			":cleanjerk": &types.AttributeValueMemberN{Value: strconv.FormatFloat(row.CleanJerk, 'f', -1, 64)},
			":total":     &types.AttributeValueMemberN{Value: strconv.FormatFloat(row.Total, 'f', -1, 64)},
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
	if !isValidLift(r.Snatch) {
		return errSnatchRange
	}
	if !isValidLift(r.CleanJerk) {
		return errCleanJerkRange
	}
	return nil
}

func decodeRowPayload(w http.ResponseWriter, r *http.Request, payload *rowPayload) error {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		return errInvalidJSON
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(payload); err != nil {
		return errInvalidJSON
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errInvalidJSON
	}
	return nil
}

func isValidLift(lift float64) bool {
	if math.IsNaN(lift) || math.IsInf(lift, 0) || lift < 0 || lift > 500 {
		return false
	}

	scaled := lift * 100
	return math.Abs(scaled-math.Round(scaled)) < 1e-9
}
