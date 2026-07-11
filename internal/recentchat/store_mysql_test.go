package recentchat

import (
	"strings"
	"testing"
)

var _ MessageStore = (*MySQLMessageStore)(nil)

func TestMySQLRecentQueryScopesBySessionAndUser(t *testing.T) {
	if !strings.Contains(listRecentBySessionUserQuery, "WHERE session_id = ? AND user_id = ?") {
		t.Fatalf("recent query is not user scoped: %q", listRecentBySessionUserQuery)
	}
}
