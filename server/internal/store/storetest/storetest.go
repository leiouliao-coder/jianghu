// Package storetest provisions throwaway PostgreSQL databases for tests.
package storetest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

// DatabaseURL creates a fresh database on the server named by
// JIANGHU_TEST_DATABASE_URL and drops it when the test ends. Without that
// variable the test is skipped, unless JIANGHU_REQUIRE_DB=1 (set in CI)
// makes it a failure.
func DatabaseURL(t testing.TB) string {
	t.Helper()
	base := os.Getenv("JIANGHU_TEST_DATABASE_URL")
	if base == "" {
		if os.Getenv("JIANGHU_REQUIRE_DB") == "1" {
			t.Fatal("JIANGHU_REQUIRE_DB=1 but JIANGHU_TEST_DATABASE_URL is empty")
		}
		t.Skip("JIANGHU_TEST_DATABASE_URL not set; skipping PostgreSQL integration test")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	name := "jianghu_t_" + hex.EncodeToString(b)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	// Registered first, so it runs after cleanups that close pools.
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
		_ = admin.Close(ctx)
	})
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	return u.String()
}
