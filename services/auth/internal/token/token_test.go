package token

import (
	"testing"
	"time"
)

func TestIssueVerifyRoundtrip(t *testing.T) {
	secret := []byte("s")
	tok, err := Issue(secret, "alice", []string{"ROLE_EDITOR"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	user, roles, err := Verify(secret, tok)
	if err != nil || user != "alice" || len(roles) != 1 || roles[0] != "ROLE_EDITOR" {
		t.Fatalf("verify = %s %v %v", user, roles, err)
	}
	if _, _, err := Verify([]byte("other"), tok); err == nil {
		t.Fatal("wrong secret must fail")
	}
	expired, _ := Issue(secret, "alice", nil, -time.Minute)
	if _, _, err := Verify(secret, expired); err == nil {
		t.Fatal("expired must fail")
	}
}
