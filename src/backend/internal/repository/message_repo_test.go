package repository

import "testing"

func TestMergeContactRows_DeduplicatesAndExcludesSelf(t *testing.T) {
	domainRows := []contactRow{
		{UserID: 1, Username: "self", Nickname: "Self", SourceID: 10, SourceName: "Domain A"},
		{UserID: 2, Username: "bob", Nickname: "Bob", SourceID: 10, SourceName: "Domain A"},
		{UserID: 2, Username: "bob", Nickname: "Bob", SourceID: 10, SourceName: "Domain A"}, // duplicate
		{UserID: 3, Username: "charlie", Nickname: "Charlie", SourceID: 11, SourceName: "Domain B"},
	}
	orgRows := []contactRow{
		{UserID: 1, Username: "self", Nickname: "Self", SourceID: 20, SourceName: "Org X"},
		{UserID: 2, Username: "bob", Nickname: "Bob", SourceID: 20, SourceName: "Org X"},
		{UserID: 4, Username: "alice", Nickname: "Alice", SourceID: 21, SourceName: "Org Y"},
	}

	got := mergeContactRows(1, domainRows, orgRows)
	if len(got) != 3 {
		t.Fatalf("expected 3 contacts, got %d: %+v", len(got), got)
	}

	// Sorted alphabetically by Nickname: Alice (4), Bob (2), Charlie (3)
	if got[0].ID != 4 || len(got[0].Sources) != 1 || got[0].Sources[0].Type != "org" || got[0].Sources[0].Name != "Org Y" {
		t.Fatalf("unexpected first contact: %+v", got[0])
	}
	if got[1].ID != 2 || len(got[1].Sources) != 2 || got[1].Sources[0].Type != "domain" || got[1].Sources[0].Name != "Domain A" || got[1].Sources[1].Type != "org" || got[1].Sources[1].Name != "Org X" {
		t.Fatalf("unexpected second contact: %+v", got[1])
	}
	if got[2].ID != 3 || len(got[2].Sources) != 1 || got[2].Sources[0].Type != "domain" {
		t.Fatalf("unexpected third contact: %+v", got[2])
	}
}
