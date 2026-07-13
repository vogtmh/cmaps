package auth

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	hash, salt := HashPassword("s3cret")
	if hash == "" || salt == "" {
		t.Fatalf("HashPassword returned empty hash or salt")
	}
	if !CheckPassword("s3cret", hash, salt) {
		t.Errorf("CheckPassword rejected the correct password")
	}
	if CheckPassword("wrong", hash, salt) {
		t.Errorf("CheckPassword accepted a wrong password")
	}
	if CheckPassword("s3cret", hash, "othersalt") {
		t.Errorf("CheckPassword accepted a wrong salt")
	}
	if CheckPassword("", hash, salt) {
		t.Errorf("CheckPassword accepted an empty password")
	}
}

func TestHashPasswordUniqueSalts(t *testing.T) {
	h1, s1 := HashPassword("same")
	h2, s2 := HashPassword("same")
	if s1 == s2 {
		t.Errorf("two HashPassword calls produced the same salt")
	}
	if h1 == h2 {
		t.Errorf("two HashPassword calls produced the same hash (salt not applied?)")
	}
}
