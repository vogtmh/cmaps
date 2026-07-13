package main

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	hash, salt := hashPassword("s3cret")
	if hash == "" || salt == "" {
		t.Fatalf("hashPassword returned empty hash or salt")
	}
	if !checkPassword("s3cret", hash, salt) {
		t.Errorf("checkPassword rejected the correct password")
	}
	if checkPassword("wrong", hash, salt) {
		t.Errorf("checkPassword accepted a wrong password")
	}
	if checkPassword("s3cret", hash, "othersalt") {
		t.Errorf("checkPassword accepted a wrong salt")
	}
	if checkPassword("", hash, salt) {
		t.Errorf("checkPassword accepted an empty password")
	}
}

func TestHashPasswordUniqueSalts(t *testing.T) {
	h1, s1 := hashPassword("same")
	h2, s2 := hashPassword("same")
	if s1 == s2 {
		t.Errorf("two hashPassword calls produced the same salt")
	}
	if h1 == h2 {
		t.Errorf("two hashPassword calls produced the same hash (salt not applied?)")
	}
}
