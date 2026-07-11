package sessionsummary

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

var (
	// ErrVersionConflict means another writer changed or created the summary.
	ErrVersionConflict = errors.New("session summary version conflict")
	// ErrWatermarkRegression prevents an update from forgetting covered messages.
	ErrWatermarkRegression = errors.New("session summary watermark cannot move backwards")
)

// SummaryStore exposes the read-and-compare-save behavior used by the updater.
type SummaryStore interface {
	Get(sessionID, userID string) (SessionSummary, bool, error)
	Save(next SessionSummary, expectedVersion int64) (SessionSummary, error)
}

// SummaryQueries keeps SQL details outside Store and gives tests a small fake boundary.
type SummaryQueries interface {
	Find(sessionID, userID string) (SessionSummary, error)
	Insert(next SessionSummary) (int64, error)
	Update(next SessionSummary, expectedVersion int64) (int64, error)
}

type Store struct {
	queries SummaryQueries
	now     func() time.Time
}

func NewStore(queries SummaryQueries) *Store {
	return &Store{queries: queries, now: time.Now}
}

func (s *Store) Get(sessionID, userID string) (SessionSummary, bool, error) {
	if err := validateSummaryKey(sessionID, userID); err != nil {
		return SessionSummary{}, false, err
	}
	if s == nil || s.queries == nil {
		return SessionSummary{}, false, fmt.Errorf("summary queries are required")
	}

	summary, err := s.queries.Find(sessionID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionSummary{}, false, nil
	}
	if err != nil {
		return SessionSummary{}, false, fmt.Errorf("get session summary: %w", err)
	}
	return summary, true, nil
}

func (s *Store) Save(next SessionSummary, expectedVersion int64) (SessionSummary, error) {
	if s == nil || s.queries == nil {
		return SessionSummary{}, fmt.Errorf("summary queries are required")
	}
	if err := validateSummaryKey(next.SessionID, next.UserID); err != nil {
		return SessionSummary{}, err
	}
	if strings.TrimSpace(next.Content) == "" {
		return SessionSummary{}, fmt.Errorf("summary content is required")
	}
	if next.LastMessageID <= 0 {
		return SessionSummary{}, fmt.Errorf("summary watermark must be positive: %d", next.LastMessageID)
	}
	if expectedVersion < 0 {
		return SessionSummary{}, fmt.Errorf("expected version must not be negative: %d", expectedVersion)
	}

	if expectedVersion == 0 {
		return s.insertFirst(next)
	}
	return s.updateExisting(next, expectedVersion)
}

func (s *Store) insertFirst(next SessionSummary) (SessionSummary, error) {
	now := s.now().UTC()
	next.Version = 1
	next.CreatedAt = now
	next.UpdatedAt = now

	affected, err := s.queries.Insert(next)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return SessionSummary{}, fmt.Errorf("%w: summary already exists", ErrVersionConflict)
		}
		return SessionSummary{}, fmt.Errorf("insert session summary: %w", err)
	}
	if affected != 1 {
		return SessionSummary{}, fmt.Errorf("%w: insert affected %d rows", ErrVersionConflict, affected)
	}
	return next, nil
}

func (s *Store) updateExisting(next SessionSummary, expectedVersion int64) (SessionSummary, error) {
	// This read provides a clear stale-version or watermark error. The SQL UPDATE
	// repeats both guards because another writer can commit after this check.
	current, err := s.queries.Find(next.SessionID, next.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionSummary{}, fmt.Errorf("%w: summary does not exist", ErrVersionConflict)
	}
	if err != nil {
		return SessionSummary{}, fmt.Errorf("get summary before update: %w", err)
	}
	if current.Version != expectedVersion {
		return SessionSummary{}, fmt.Errorf("%w: expected %d, found %d", ErrVersionConflict, expectedVersion, current.Version)
	}
	if next.LastMessageID < current.LastMessageID {
		return SessionSummary{}, fmt.Errorf("%w: current %d, next %d", ErrWatermarkRegression, current.LastMessageID, next.LastMessageID)
	}

	next.Version = expectedVersion + 1
	next.CreatedAt = current.CreatedAt
	next.UpdatedAt = s.now().UTC()
	affected, err := s.queries.Update(next, expectedVersion)
	if err != nil {
		return SessionSummary{}, fmt.Errorf("update session summary: %w", err)
	}
	if affected != 1 {
		return SessionSummary{}, fmt.Errorf("%w: update affected %d rows", ErrVersionConflict, affected)
	}
	return next, nil
}

func validateSummaryKey(sessionID, userID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session ID is required")
	}
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("user ID is required")
	}
	return nil
}
