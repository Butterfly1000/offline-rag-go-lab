package memoryitem

import (
	"fmt"
	"sort"
	"strings"
)

type Action string

const (
	ActionInsert Action = "insert"
	ActionUpdate Action = "update"
	ActionNoop   Action = "noop"
	ActionForget Action = "forget"
)

type Decision struct {
	Action    Action
	Current   *Item
	Next      Item
	Candidate Candidate
	Reason    string
}

func Resolve(current *Item, candidate Candidate) (Decision, error) {
	return resolve(current, candidate, false)
}

func resolve(current *Item, candidate Candidate, allowProvisional bool) (Decision, error) {
	normalized, err := normalizeResolvedCandidate(candidate)
	if err != nil {
		return Decision{}, err
	}
	if current == nil {
		if normalized.Operation == OperationForget {
			return Decision{
				Action: ActionNoop, Candidate: normalized,
				Next:   Item{Kind: normalized.Kind, Key: normalized.Key, Status: StatusForgotten},
				Reason: "missing_item_already_forgotten",
			}, nil
		}
		return Decision{
			Action: ActionInsert, Candidate: normalized,
			Next: Item{
				Kind: normalized.Kind, Key: normalized.Key, Value: normalized.Value,
				Status: StatusActive, Version: 1,
			},
			Reason: "new_memory_item",
		}, nil
	}

	currentCopy := *current
	if err := validateCurrentItem(currentCopy, allowProvisional); err != nil {
		return Decision{}, err
	}
	if currentCopy.Kind != normalized.Kind || currentCopy.Key != normalized.Key {
		return Decision{}, fmt.Errorf(
			"candidate identity %s/%s does not match current item %s/%s",
			normalized.Kind, normalized.Key, currentCopy.Kind, currentCopy.Key,
		)
	}

	decision := Decision{Current: &currentCopy, Next: currentCopy, Candidate: normalized}
	if normalized.Operation == OperationForget {
		if currentCopy.Status == StatusForgotten {
			decision.Action = ActionNoop
			decision.Reason = "item_already_forgotten"
			return decision, nil
		}
		decision.Action = ActionForget
		decision.Next.Status = StatusForgotten
		decision.Next.Version++
		decision.Reason = "explicit_forget_request"
		return decision, nil
	}

	if currentCopy.Status == StatusActive && equivalentMemoryValue(currentCopy.Value, normalized.Value) {
		decision.Action = ActionNoop
		decision.Reason = "equivalent_active_value"
		return decision, nil
	}
	decision.Action = ActionUpdate
	decision.Next.Value = normalized.Value
	decision.Next.Status = StatusActive
	decision.Next.Version++
	if currentCopy.Status == StatusForgotten {
		decision.Reason = "restore_forgotten_item"
	} else {
		decision.Reason = "replace_changed_value"
	}
	return decision, nil
}

func ResolveBatch(current map[string]Item, candidates []Candidate) ([]Decision, error) {
	type orderedCandidate struct {
		candidate Candidate
		position  int
		sourceID  int64
	}
	ordered := make([]orderedCandidate, 0, len(candidates))
	for position, candidate := range candidates {
		normalized, err := normalizeResolvedCandidate(candidate)
		if err != nil {
			return nil, fmt.Errorf("candidate %d: %w", position, err)
		}
		sourceID, err := minimumCandidateSourceID(normalized)
		if err != nil {
			return nil, fmt.Errorf("candidate %d: %w", position, err)
		}
		ordered = append(ordered, orderedCandidate{candidate: normalized, position: position, sourceID: sourceID})
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].sourceID == ordered[j].sourceID {
			return ordered[i].position < ordered[j].position
		}
		return ordered[i].sourceID < ordered[j].sourceID
	})

	state := make(map[string]Item, len(current)+len(candidates))
	for _, item := range current {
		if err := validateCurrentItem(item, false); err != nil {
			return nil, err
		}
		key := itemIdentity(item.Kind, item.Key)
		if _, exists := state[key]; exists {
			return nil, fmt.Errorf("duplicate current item identity %s/%s", item.Kind, item.Key)
		}
		state[key] = item
	}
	provisional := make(map[string]bool)
	decisions := make([]Decision, 0, len(ordered))
	for _, entry := range ordered {
		identity := itemIdentity(entry.candidate.Kind, entry.candidate.Key)
		item, exists := state[identity]
		var currentItem *Item
		if exists {
			copy := item
			currentItem = &copy
		}
		decision, err := resolve(currentItem, entry.candidate, provisional[identity])
		if err != nil {
			return nil, fmt.Errorf("resolve candidate %d: %w", entry.position, err)
		}
		decisions = append(decisions, decision)
		switch decision.Action {
		case ActionInsert:
			state[identity] = decision.Next
			provisional[identity] = true
		case ActionUpdate, ActionForget:
			state[identity] = decision.Next
		}
	}
	return decisions, nil
}

func normalizeResolvedCandidate(candidate Candidate) (Candidate, error) {
	operation, err := normalizeOperation(candidate.Operation)
	if err != nil {
		return Candidate{}, err
	}
	kind, err := normalizeKind(candidate.Kind)
	if err != nil {
		return Candidate{}, err
	}
	key, err := normalizeMemoryKey(candidate.Key)
	if err != nil {
		return Candidate{}, err
	}
	value := strings.TrimSpace(candidate.Value)
	if operation == OperationUpsert && value == "" {
		return Candidate{}, fmt.Errorf("upsert value is required")
	}
	if _, err := minimumCandidateSourceID(candidate); err != nil {
		return Candidate{}, err
	}
	candidate.Operation = operation
	candidate.Kind = kind
	candidate.Key = key
	candidate.Value = value
	return candidate, nil
}

func validateCurrentItem(item Item, allowProvisional bool) error {
	if item.ID <= 0 && !(allowProvisional && item.ID == 0) {
		return fmt.Errorf("current item ID must be positive: %d", item.ID)
	}
	if item.Version <= 0 {
		return fmt.Errorf("current item version must be positive: %d", item.Version)
	}
	if item.Status != StatusActive && item.Status != StatusForgotten {
		return fmt.Errorf("unsupported current item status %q", item.Status)
	}
	return nil
}

func minimumCandidateSourceID(candidate Candidate) (int64, error) {
	if len(candidate.SourceMessageIDs) == 0 {
		return 0, fmt.Errorf("candidate source message IDs are required")
	}
	minimum := candidate.SourceMessageIDs[0]
	if minimum <= 0 {
		return 0, fmt.Errorf("candidate source message ID must be positive: %d", minimum)
	}
	for _, sourceID := range candidate.SourceMessageIDs[1:] {
		if sourceID <= 0 {
			return 0, fmt.Errorf("candidate source message ID must be positive: %d", sourceID)
		}
		if sourceID < minimum {
			minimum = sourceID
		}
	}
	return minimum, nil
}

func equivalentMemoryValue(left, right string) bool {
	normalize := func(value string) string {
		return strings.Join(strings.Fields(value), " ")
	}
	return strings.EqualFold(normalize(left), normalize(right))
}

func itemIdentity(kind Kind, key string) string {
	return string(kind) + "\x00" + key
}
