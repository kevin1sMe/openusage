package shared

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/janekbaraniewski/openusage/internal/browsercookies"
	"github.com/janekbaraniewski/openusage/internal/config"
	"github.com/janekbaraniewski/openusage/internal/core"
)

func TestLoadOrRefreshBrowserSessionFrom_RefreshesStoredSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	if err := config.SaveSessionTo(path, "perplexity", config.BrowserSession{
		Domain:        ".perplexity.ai",
		CookieName:    "__Secure-next-auth.session-token",
		Value:         "old-cookie",
		SourceBrowser: "firefox",
		CapturedAt:    "2026-04-30T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	reader := &browsercookies.FakeReader{
		Cookies: []browsercookies.Cookie{{
			Name:    "__Secure-next-auth.session-token",
			Value:   "fresh-cookie",
			Domain:  ".perplexity.ai",
			Source:  "chrome",
			Expires: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
		}},
	}
	acct := core.AccountConfig{
		ID:       "perplexity",
		Provider: "perplexity",
		BrowserCookie: &core.BrowserCookieRef{
			Domain:        ".perplexity.ai",
			CookieName:    "__Secure-next-auth.session-token",
			SourceBrowser: "chrome",
		},
	}

	session, ok, err := loadOrRefreshBrowserSessionFrom(path, context.Background(), acct, reader)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("session not found after refresh")
	}
	if session.Value != "fresh-cookie" {
		t.Fatalf("Value = %q, want fresh-cookie", session.Value)
	}
	if session.SourceBrowser != "chrome" {
		t.Fatalf("SourceBrowser = %q, want chrome", session.SourceBrowser)
	}

	saved, ok, err := config.LoadSessionFrom(path, "perplexity")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("session not saved after refresh")
	}
	if saved.Value != "fresh-cookie" {
		t.Fatalf("saved session value = %q, want fresh-cookie", saved.Value)
	}
}

func TestLoadOrRefreshBrowserSessionFrom_FallsBackToStoredOnNoCookie(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	stored := config.BrowserSession{
		Domain:        ".opencode.ai",
		CookieName:    "auth",
		Value:         "stored-cookie",
		SourceBrowser: "chrome",
		CapturedAt:    "2026-04-30T00:00:00Z",
	}
	if err := config.SaveSessionTo(path, "opencode", stored); err != nil {
		t.Fatal(err)
	}

	reader := &browsercookies.FakeReader{Err: browsercookies.ErrNoCookieFound}
	acct := core.AccountConfig{
		ID:       "opencode",
		Provider: "opencode",
		BrowserCookie: &core.BrowserCookieRef{
			Domain:        ".opencode.ai",
			CookieName:    "auth",
			SourceBrowser: "chrome",
		},
	}

	session, ok, err := loadOrRefreshBrowserSessionFrom(path, context.Background(), acct, reader)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected stored fallback session")
	}
	if session != stored {
		t.Fatalf("session = %+v, want %+v", session, stored)
	}
}

func TestLoadOrRefreshBrowserSessionFrom_UsesStoredRefWhenAccountMissingBrowserCookie(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	if err := config.SaveSessionTo(path, "perplexity", config.BrowserSession{
		Domain:        ".perplexity.ai",
		CookieName:    "__Secure-next-auth.session-token",
		Value:         "stored-cookie",
		SourceBrowser: "safari",
	}); err != nil {
		t.Fatal(err)
	}

	reader := &browsercookies.FakeReader{
		Cookies: []browsercookies.Cookie{{
			Name:   "__Secure-next-auth.session-token",
			Value:  "fresh-cookie",
			Domain: ".perplexity.ai",
			Source: "safari",
		}},
	}
	acct := core.AccountConfig{ID: "perplexity", Provider: "perplexity"}

	session, ok, err := loadOrRefreshBrowserSessionFrom(path, context.Background(), acct, reader)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected refreshed session")
	}
	if session.Value != "fresh-cookie" {
		t.Fatalf("Value = %q, want fresh-cookie", session.Value)
	}
}

func TestLoadOrRefreshBrowserSessionFrom_PropagatesReadErrorWithoutStoredFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	reader := &browsercookies.FakeReader{Err: errors.New("keychain denied")}
	acct := core.AccountConfig{
		ID:       "perplexity",
		Provider: "perplexity",
		BrowserCookie: &core.BrowserCookieRef{
			Domain:     ".perplexity.ai",
			CookieName: "__Secure-next-auth.session-token",
		},
	}

	_, ok, err := loadOrRefreshBrowserSessionFrom(path, context.Background(), acct, reader)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if ok {
		t.Fatal("ok = true, want false")
	}
}
