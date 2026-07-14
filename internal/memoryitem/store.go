package memoryitem

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

var ErrMemoryVersionConflict = errors.New("memory item version conflict")

type Evidence struct {
	ItemID    int64
	UserID    string
	SessionID string
	MessageID int64
	Role      string
	Operation Operation
	Text      string
	CreatedAt time.Time
}

type ApplyRequest struct {
	UserID         string
	SessionID      string
	Candidate      Candidate
	SourceMessages []SourceMessage
}

type ApplyResult struct {
	Decision         Decision
	Item             Item
	EvidenceInserted int
}

type MemoryStore interface {
	Get(ctx context.Context, userID string, kind Kind, key string) (Item, bool, error)
	Apply(ctx context.Context, req ApplyRequest) (ApplyResult, error)
	ListActive(ctx context.Context, userID string) ([]Item, error)
}

type MemoryReader interface {
	Find(ctx context.Context, userID string, kind Kind, key string) (Item, error)
	ListActive(ctx context.Context, userID string) ([]Item, error)
}

type MemoryUnitOfWork interface {
	FindForUpdate(ctx context.Context, userID string, kind Kind, key string) (Item, error)
	InsertItem(ctx context.Context, item Item) (int64, error)
	UpdateItem(ctx context.Context, item Item, expectedVersion int64) (int64, error)
	InsertEvidence(ctx context.Context, evidence Evidence) (int64, error)
	Commit() error
	Rollback() error
}

type MemoryTransactionFactory interface {
	Begin(ctx context.Context) (MemoryUnitOfWork, error)
}

type Store struct {
	reader       MemoryReader
	transactions MemoryTransactionFactory
	now          func() time.Time
}

func NewStore(reader MemoryReader, transactions MemoryTransactionFactory) *Store {
	return &Store{reader: reader, transactions: transactions, now: time.Now}
}

func (s *Store) Get(ctx context.Context, userID string, kind Kind, key string) (Item, bool, error) {
	if s == nil || s.reader == nil {
		return Item{}, false, fmt.Errorf("memory reader is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return Item{}, false, fmt.Errorf("user ID is required")
	}
	normalizedKind, err := normalizeKind(kind)
	if err != nil {
		return Item{}, false, err
	}
	normalizedKey, err := normalizeMemoryKey(key)
	if err != nil {
		return Item{}, false, err
	}
	item, err := s.reader.Find(ctx, userID, normalizedKind, normalizedKey)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, false, nil
	}
	if err != nil {
		return Item{}, false, fmt.Errorf("get memory item: %w", err)
	}
	if item.UserID != userID {
		return Item{}, false, fmt.Errorf("memory reader returned item for user %q, want %q", item.UserID, userID)
	}
	return item, true, nil
}

func (s *Store) ListActive(ctx context.Context, userID string) ([]Item, error) {
	if s == nil || s.reader == nil {
		return nil, fmt.Errorf("memory reader is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user ID is required")
	}
	items, err := s.reader.ListActive(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list active memory items: %w", err)
	}
	for _, item := range items {
		if item.UserID != userID || item.Status != StatusActive {
			return nil, fmt.Errorf("memory reader returned item %d outside active user scope", item.ID)
		}
	}
	return items, nil
}

func (s *Store) Apply(ctx context.Context, req ApplyRequest) (result ApplyResult, err error) {
	if s == nil || s.transactions == nil {
		return ApplyResult{}, fmt.Errorf("memory transaction factory is required")
	}
	normalized, err := ValidateAndNormalizeCandidate(req.UserID, req.SessionID, req.Candidate, req.SourceMessages)
	if err != nil {
		return ApplyResult{}, err
	}
	req.UserID = strings.TrimSpace(req.UserID)
	req.SessionID = strings.TrimSpace(req.SessionID)

	tx, err := s.transactions.Begin(ctx)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("begin memory transaction: %w", err)
	}
	if tx == nil {
		return ApplyResult{}, fmt.Errorf("memory transaction is required")
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	current, findErr := tx.FindForUpdate(ctx, req.UserID, normalized.Kind, normalized.Key)
	var currentPtr *Item
	if errors.Is(findErr, sql.ErrNoRows) {
		currentPtr = nil
	} else if findErr != nil {
		return ApplyResult{}, fmt.Errorf("lock memory item: %w", findErr)
	} else {
		if current.UserID != req.UserID {
			return ApplyResult{}, fmt.Errorf("locked memory item belongs to user %q, want %q", current.UserID, req.UserID)
		}
		currentPtr = &current
	}

	decision, err := Resolve(currentPtr, normalized)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("resolve memory item: %w", err)
	}
	next, err := s.persistDecision(ctx, tx, req.UserID, decision)
	if err != nil {
		return ApplyResult{}, err
	}
	decision.Next = next

	evidenceInserted := 0
	if next.ID > 0 {
		sources, sourceErr := indexSourceMessages(req.SourceMessages)
		if sourceErr != nil {
			return ApplyResult{}, sourceErr
		}
		for _, sourceID := range normalized.SourceMessageIDs {
			source := sources[sourceID]
			affected, insertErr := tx.InsertEvidence(ctx, Evidence{
				ItemID: next.ID, UserID: req.UserID, SessionID: req.SessionID,
				MessageID: sourceID, Role: strings.ToLower(strings.TrimSpace(source.Role)),
				Operation: normalized.Operation, Text: source.Content, CreatedAt: s.now().UTC(),
			})
			if insertErr != nil {
				return ApplyResult{}, fmt.Errorf("insert memory evidence for message %d: %w", sourceID, insertErr)
			}
			if affected < 0 || affected > 1 {
				return ApplyResult{}, fmt.Errorf("insert memory evidence affected %d rows", affected)
			}
			evidenceInserted += int(affected)
		}
	}

	if err := tx.Commit(); err != nil {
		return ApplyResult{}, fmt.Errorf("commit memory transaction: %w", err)
	}
	committed = true
	return ApplyResult{Decision: decision, Item: next, EvidenceInserted: evidenceInserted}, nil
}

func (s *Store) persistDecision(ctx context.Context, tx MemoryUnitOfWork, userID string, decision Decision) (Item, error) {
	next := decision.Next
	now := s.now().UTC()
	switch decision.Action {
	case ActionInsert:
		next.UserID = userID
		next.CreatedAt = now
		next.UpdatedAt = now
		itemID, err := tx.InsertItem(ctx, next)
		if err != nil {
			if isDuplicateKey(err) {
				return Item{}, fmt.Errorf("%w: item already exists", ErrMemoryVersionConflict)
			}
			return Item{}, fmt.Errorf("insert memory item: %w", err)
		}
		if itemID <= 0 {
			return Item{}, fmt.Errorf("insert memory item returned invalid ID %d", itemID)
		}
		next.ID = itemID
		return next, nil
	case ActionUpdate, ActionForget:
		next.UserID = userID
		next.UpdatedAt = now
		affected, err := tx.UpdateItem(ctx, next, decision.Current.Version)
		if err != nil {
			return Item{}, fmt.Errorf("update memory item: %w", err)
		}
		if affected != 1 {
			return Item{}, fmt.Errorf("%w: update affected %d rows", ErrMemoryVersionConflict, affected)
		}
		return next, nil
	case ActionNoop:
		return next, nil
	default:
		return Item{}, fmt.Errorf("unsupported memory action %q", decision.Action)
	}
}

func isDuplicateKey(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
