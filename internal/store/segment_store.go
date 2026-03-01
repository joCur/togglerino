package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type SegmentStore struct {
	pool *pgxpool.Pool
}

func NewSegmentStore(pool *pgxpool.Pool) *SegmentStore {
	return &SegmentStore{pool: pool}
}

// Create inserts a new segment and returns it.
func (s *SegmentStore) Create(ctx context.Context, projectID, key, name, description string, conditions json.RawMessage) (*model.Segment, error) {
	var seg model.Segment
	var conditionsJSON []byte
	err := s.pool.QueryRow(ctx,
		`INSERT INTO segments (project_id, key, name, description, conditions)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, project_id, key, name, description, conditions, created_at, updated_at`,
		projectID, key, name, description, conditions,
	).Scan(&seg.ID, &seg.ProjectID, &seg.Key, &seg.Name, &seg.Description, &conditionsJSON, &seg.CreatedAt, &seg.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating segment: %w", err)
	}
	if err := json.Unmarshal(conditionsJSON, &seg.Conditions); err != nil {
		return nil, fmt.Errorf("unmarshaling segment conditions: %w", err)
	}
	if seg.Conditions == nil {
		seg.Conditions = []model.Condition{}
	}
	return &seg, nil
}

// GetByKey returns a segment by project ID and segment key.
func (s *SegmentStore) GetByKey(ctx context.Context, projectID, key string) (*model.Segment, error) {
	var seg model.Segment
	var conditionsJSON []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, key, name, description, conditions, created_at, updated_at
		 FROM segments WHERE project_id = $1 AND key = $2`,
		projectID, key,
	).Scan(&seg.ID, &seg.ProjectID, &seg.Key, &seg.Name, &seg.Description, &conditionsJSON, &seg.CreatedAt, &seg.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("finding segment by key: %w", err)
	}
	if err := json.Unmarshal(conditionsJSON, &seg.Conditions); err != nil {
		return nil, fmt.Errorf("unmarshaling segment conditions: %w", err)
	}
	if seg.Conditions == nil {
		seg.Conditions = []model.Condition{}
	}
	return &seg, nil
}

// ListByProject returns all segments for a project, ordered by created_at DESC.
func (s *SegmentStore) ListByProject(ctx context.Context, projectID string) ([]model.Segment, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, key, name, description, conditions, created_at, updated_at
		 FROM segments WHERE project_id = $1 ORDER BY created_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing segments: %w", err)
	}
	defer rows.Close()

	var segments []model.Segment
	for rows.Next() {
		var seg model.Segment
		var conditionsJSON []byte
		if err := rows.Scan(&seg.ID, &seg.ProjectID, &seg.Key, &seg.Name, &seg.Description, &conditionsJSON, &seg.CreatedAt, &seg.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning segment: %w", err)
		}
		if err := json.Unmarshal(conditionsJSON, &seg.Conditions); err != nil {
			return nil, fmt.Errorf("unmarshaling segment conditions: %w", err)
		}
		if seg.Conditions == nil {
			seg.Conditions = []model.Condition{}
		}
		segments = append(segments, seg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating segments: %w", err)
	}
	return segments, nil
}

// Update updates a segment's name, description, and conditions.
func (s *SegmentStore) Update(ctx context.Context, segmentID, name, description string, conditions json.RawMessage) (*model.Segment, error) {
	var seg model.Segment
	var conditionsJSON []byte
	err := s.pool.QueryRow(ctx,
		`UPDATE segments SET name=$2, description=$3, conditions=$4, updated_at=NOW() WHERE id=$1
		 RETURNING id, project_id, key, name, description, conditions, created_at, updated_at`,
		segmentID, name, description, conditions,
	).Scan(&seg.ID, &seg.ProjectID, &seg.Key, &seg.Name, &seg.Description, &conditionsJSON, &seg.CreatedAt, &seg.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating segment: %w", err)
	}
	if err := json.Unmarshal(conditionsJSON, &seg.Conditions); err != nil {
		return nil, fmt.Errorf("unmarshaling segment conditions: %w", err)
	}
	if seg.Conditions == nil {
		seg.Conditions = []model.Condition{}
	}
	return &seg, nil
}

// Delete deletes a segment by ID.
func (s *SegmentStore) Delete(ctx context.Context, segmentID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM segments WHERE id = $1`, segmentID)
	if err != nil {
		return fmt.Errorf("deleting segment: %w", err)
	}
	return nil
}

// ListByProjectKey returns all segments for a project looked up by project key.
// Used by the evaluation cache to load segments.
func (s *SegmentStore) ListByProjectKey(ctx context.Context, projectKey string) ([]model.Segment, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT s.id, s.project_id, s.key, s.name, s.description, s.conditions, s.created_at, s.updated_at
		 FROM segments s
		 JOIN projects p ON p.id = s.project_id
		 WHERE p.key = $1
		 ORDER BY s.created_at DESC`,
		projectKey,
	)
	if err != nil {
		return nil, fmt.Errorf("listing segments by project key: %w", err)
	}
	defer rows.Close()

	var segments []model.Segment
	for rows.Next() {
		var seg model.Segment
		var conditionsJSON []byte
		if err := rows.Scan(&seg.ID, &seg.ProjectID, &seg.Key, &seg.Name, &seg.Description, &conditionsJSON, &seg.CreatedAt, &seg.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning segment: %w", err)
		}
		if err := json.Unmarshal(conditionsJSON, &seg.Conditions); err != nil {
			return nil, fmt.Errorf("unmarshaling segment conditions: %w", err)
		}
		if seg.Conditions == nil {
			seg.Conditions = []model.Condition{}
		}
		segments = append(segments, seg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating segments: %w", err)
	}
	return segments, nil
}

// FindReferencingFlags returns the keys of flags that have targeting rules
// containing a segment_match condition referencing the given segment key.
// Uses JSONB containment to search flag_environment_configs.
func (s *SegmentStore) FindReferencingFlags(ctx context.Context, projectID, segmentKey string) ([]string, error) {
	// Build the JSONB containment pattern using json.Marshal for safe encoding:
	// [{"conditions":[{"operator":"segment_match","value":"<segmentKey>"}]}]
	type condPattern struct {
		Operator string `json:"operator"`
		Value    string `json:"value"`
	}
	type rulePattern struct {
		Conditions []condPattern `json:"conditions"`
	}
	pattern, err := json.Marshal([]rulePattern{{Conditions: []condPattern{{Operator: "segment_match", Value: segmentKey}}}})
	if err != nil {
		return nil, fmt.Errorf("marshaling segment reference pattern: %w", err)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT f.key
		 FROM flag_environment_configs fec
		 JOIN flags f ON f.id = fec.flag_id
		 WHERE f.project_id = $1
		 AND fec.targeting_rules @> $2::jsonb`,
		projectID, pattern,
	)
	if err != nil {
		return nil, fmt.Errorf("finding referencing flags: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scanning flag key: %w", err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating flag keys: %w", err)
	}
	return keys, nil
}
