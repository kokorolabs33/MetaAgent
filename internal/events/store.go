package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/models"
)

type Store struct {
	DB *pgxpool.Pool
}

// Save persists an event to the database and returns the saved event with generated ID and timestamp.
func (s *Store) Save(ctx context.Context, taskID, subtaskID, eventType, actorType, actorID string, data any) (*models.Event, error) {
	return s.SaveWithConversation(ctx, taskID, "", subtaskID, eventType, actorType, actorID, data)
}

// SaveWithConversation persists an event with an explicit conversation_id.
func (s *Store) SaveWithConversation(ctx context.Context, taskID, conversationID, subtaskID, eventType, actorType, actorID string, data any) (*models.Event, error) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal event data: %w", err)
	}

	evt := &models.Event{
		ID:             uuid.New().String(),
		TaskID:         taskID,
		ConversationID: conversationID,
		SubtaskID:      subtaskID,
		Type:           eventType,
		ActorType:      actorType,
		ActorID:        actorID,
		Data:           dataJSON,
		CreatedAt:      time.Now(),
	}

	_, err = s.DB.Exec(ctx,
		`INSERT INTO events (id, task_id, conversation_id, subtask_id, type, actor_type, actor_id, data, created_at)
		 VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, $6, $7, $8, $9)`,
		evt.ID, evt.TaskID, evt.ConversationID, evt.SubtaskID, evt.Type, evt.ActorType, evt.ActorID, evt.Data, evt.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert event: %w", err)
	}

	return evt, nil
}

// ListByTask returns all events for a task, ordered by created_at.
func (s *Store) ListByTask(ctx context.Context, taskID string) ([]models.Event, error) {
	return s.queryEvents(ctx, taskID, time.Time{}, "")
}

// ListByTaskAfter returns events after a given (created_at, id) pair for SSE catchup.
func (s *Store) ListByTaskAfter(ctx context.Context, taskID string, afterTime time.Time, afterID string) ([]models.Event, error) {
	return s.queryEvents(ctx, taskID, afterTime, afterID)
}

func (s *Store) queryEvents(ctx context.Context, taskID string, afterTime time.Time, afterID string) ([]models.Event, error) {
	var rows pgx.Rows
	var err error

	if afterID != "" {
		rows, err = s.DB.Query(ctx,
			`SELECT id, task_id, COALESCE(subtask_id, ''), type, actor_type, COALESCE(actor_id, ''), data, created_at
			 FROM events
			 WHERE task_id = $1 AND (created_at, id) > ($2, $3)
			 ORDER BY created_at, id`,
			taskID, afterTime, afterID)
	} else {
		rows, err = s.DB.Query(ctx,
			`SELECT id, task_id, COALESCE(subtask_id, ''), type, actor_type, COALESCE(actor_id, ''), data, created_at
			 FROM events
			 WHERE task_id = $1
			 ORDER BY created_at, id`,
			taskID)
	}
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var e models.Event
		if err := rows.Scan(&e.ID, &e.TaskID, &e.SubtaskID, &e.Type, &e.ActorType, &e.ActorID, &e.Data, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return events, nil
}

// ListByConversation returns all events for a conversation, ordered by created_at.
func (s *Store) ListByConversation(ctx context.Context, conversationID string) ([]models.Event, error) {
	return s.queryConversationEvents(ctx, conversationID, time.Time{}, "")
}

// ListByConversationAfter returns events after a given (created_at, id) pair for SSE catchup.
func (s *Store) ListByConversationAfter(ctx context.Context, conversationID string, afterTime time.Time, afterID string) ([]models.Event, error) {
	return s.queryConversationEvents(ctx, conversationID, afterTime, afterID)
}

func (s *Store) queryConversationEvents(ctx context.Context, conversationID string, afterTime time.Time, afterID string) ([]models.Event, error) {
	var rows pgx.Rows
	var err error

	if afterID != "" {
		rows, err = s.DB.Query(ctx,
			`SELECT id, task_id, COALESCE(conversation_id, ''), COALESCE(subtask_id, ''), type, actor_type, COALESCE(actor_id, ''), data, created_at
			 FROM events
			 WHERE conversation_id = $1 AND (created_at, id) > ($2, $3)
			 ORDER BY created_at, id`,
			conversationID, afterTime, afterID)
	} else {
		rows, err = s.DB.Query(ctx,
			`SELECT id, task_id, COALESCE(conversation_id, ''), COALESCE(subtask_id, ''), type, actor_type, COALESCE(actor_id, ''), data, created_at
			 FROM events
			 WHERE conversation_id = $1
			 ORDER BY created_at, id`,
			conversationID)
	}
	if err != nil {
		return nil, fmt.Errorf("query conversation events: %w", err)
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var e models.Event
		if err := rows.Scan(&e.ID, &e.TaskID, &e.ConversationID, &e.SubtaskID, &e.Type, &e.ActorType, &e.ActorID, &e.Data, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return events, nil
}

// GetByID returns a single event by ID (used for Last-Event-ID lookup).
func (s *Store) GetByID(ctx context.Context, eventID string) (*models.Event, error) {
	var e models.Event
	err := s.DB.QueryRow(ctx,
		`SELECT id, task_id, COALESCE(conversation_id, ''), COALESCE(subtask_id, ''), type, actor_type, COALESCE(actor_id, ''), data, created_at
		 FROM events WHERE id = $1`, eventID).
		Scan(&e.ID, &e.TaskID, &e.ConversationID, &e.SubtaskID, &e.Type, &e.ActorType, &e.ActorID, &e.Data, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}
