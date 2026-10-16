package auth

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	h, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("unexpected encoding %q", h)
	}
	h2, _ := HashPassword("correct horse")
	if h == h2 {
		t.Fatal("same password must get a fresh salt")
	}
	for pw, want := range map[string]bool{"correct horse": true, "correct horsE": false, "": false} {
		if ok, err := VerifyPassword(h, pw); err != nil || ok != want {
			t.Errorf("verify %q = %v, %v", pw, ok, err)
		}
	}
	for _, bad := range []string{"", "$argon2i$v=19$m=1,t=1,p=1$AA$AA", "$argon2id$v=19$m=0,t=1,p=1$AA$AA", "$argon2id$v=19$x$AA$AA", "$argon2id$v=19$m=8,t=1,p=1$!!$AA"} {
		if _, err := VerifyPassword(bad, "x"); !errors.Is(err, ErrHashFormat) {
			t.Errorf("malformed %q err=%v", bad, err)
		}
	}
}

func TestPasswordLength(t *testing.T) {
	for _, pw := range []string{"short", strings.Repeat("x", 129)} {
		if _, err := HashPassword(pw); !errors.Is(err, ErrPasswordLength) {
			t.Errorf("len %d err=%v", len(pw), err)
		}
	}
}

func TestValidName(t *testing.T) {
	for name, ok := range map[string]bool{
		"顾砚": true, "hero_01": true, "a-b": true, "x": false, "has space": false,
		"tab\t": false, "emoji😀": false, strings.Repeat("长", 32): true, strings.Repeat("长", 33): false,
		"<script>": false,
	} {
		if err := ValidName(name); (err == nil) != ok {
			t.Errorf("ValidName(%q) err=%v want ok=%v", name, err, ok)
		}
	}
}

func TestToken(t *testing.T) {
	tok, h, err := NewToken()
	if err != nil || len(tok) < 40 || !bytes.Equal(h, TokenHash(tok)) {
		t.Fatalf("token=%q err=%v", tok, err)
	}
	tok2, _, _ := NewToken()
	if tok == tok2 {
		t.Fatal("tokens must be random")
	}
}
