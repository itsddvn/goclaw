//go:build integration

package integration

import (
	"bytes"
	"os"
	"testing"
)

func TestZaloOARetypeMigration(t *testing.T) {
	up, err := os.ReadFile("../../migrations/000099_reconcile_channel_archive.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := testDB(t).Begin()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	// Shadow the real table so this data migration cannot alter another test's rows.
	if _, err := tx.Exec(`CREATE TEMP TABLE channel_instances (id text PRIMARY KEY, channel_type text, credentials bytea, config jsonb) ON COMMIT DROP`); err != nil {
		t.Fatal(err)
	}
	credentials := []byte{0, 1, 255, 42} // opaque encrypted bytes, not parsed JSON
	if _, err := tx.Exec(`INSERT INTO channel_instances VALUES
		('bot', 'zalo_oa', $1, '{"token":"legacy-bot-token"}'),
		('oauth', 'zalo_oa', $1, '{}'),
		('ambiguous', 'zalo_oa', $1, NULL),
		('personal', 'zalo_personal', $1, '{}')`, credentials); err != nil {
		t.Fatal(err)
	}
	// Reconciliation must be repeatable and never reinterpret opaque OAuth credentials.
	for range 2 {
		if _, err := tx.Exec(string(up)); err != nil {
			t.Fatal(err)
		}
		for id, want := range map[string]string{"bot": "zalo_bot", "oauth": "zalo_oa", "ambiguous": "zalo_oa", "personal": "zalo_personal"} {
			var channelType string
			var got []byte
			if err := tx.QueryRow(`SELECT channel_type, credentials FROM channel_instances WHERE id = $1`, id).Scan(&channelType, &got); err != nil {
				t.Fatal(err)
			}
			if channelType != want || !bytes.Equal(got, credentials) {
				t.Fatalf("migration corrupted %s: type=%s, credentials preserved=%t", id, channelType, bytes.Equal(got, credentials))
			}
		}
	}
}
