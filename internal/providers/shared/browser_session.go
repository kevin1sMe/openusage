package shared

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/janekbaraniewski/openusage/internal/browsercookies"
	"github.com/janekbaraniewski/openusage/internal/config"
	"github.com/janekbaraniewski/openusage/internal/core"
)

// LoadOrRefreshBrowserSession reloads the provider's session cookie from the
// user's chosen browser when possible, falling back to the last stored session
// when browser access is unavailable. This is what lets "log in again in the
// browser" repair a provider on the next poll without another TUI round-trip.
func LoadOrRefreshBrowserSession(ctx context.Context, acct core.AccountConfig, reader browsercookies.Reader) (config.BrowserSession, bool, error) {
	return loadOrRefreshBrowserSessionFrom(config.CredentialsPath(), ctx, acct, reader)
}

func loadOrRefreshBrowserSessionFrom(path string, ctx context.Context, acct core.AccountConfig, reader browsercookies.Reader) (config.BrowserSession, bool, error) {
	stored, ok, err := config.LoadSessionFrom(path, acct.ID)
	if err != nil {
		return config.BrowserSession{}, false, err
	}

	ref := acct.BrowserCookie
	if ref == nil && ok {
		ref = &core.BrowserCookieRef{
			Domain:        stored.Domain,
			CookieName:    stored.CookieName,
			SourceBrowser: stored.SourceBrowser,
		}
	}
	if ref == nil || strings.TrimSpace(ref.Domain) == "" || strings.TrimSpace(ref.CookieName) == "" {
		return stored, ok && strings.TrimSpace(stored.Value) != "", nil
	}

	if reader == nil {
		reader = browsercookies.New()
	}
	cookie, err := reader.ReadCookie(ctx, ref.Domain, ref.CookieName, ref.SourceBrowser)
	if err == nil {
		fresh := config.BrowserSession{
			Domain:        cookie.Domain,
			CookieName:    cookie.Name,
			Value:         cookie.Value,
			SourceBrowser: core.FirstNonEmpty(cookie.Source, ref.SourceBrowser),
			CapturedAt:    time.Now().UTC().Format(time.RFC3339),
		}
		if !cookie.Expires.IsZero() {
			fresh.ExpiresAt = cookie.Expires.UTC().Format(time.RFC3339)
		}
		if !ok || stored != fresh {
			if saveErr := config.SaveSessionTo(path, acct.ID, fresh); saveErr != nil {
				return config.BrowserSession{}, false, saveErr
			}
		}
		return fresh, true, nil
	}

	if ok && strings.TrimSpace(stored.Value) != "" && errors.Is(err, browsercookies.ErrNoCookieFound) {
		return stored, true, nil
	}
	if ok && strings.TrimSpace(stored.Value) != "" {
		return stored, true, nil
	}
	if errors.Is(err, browsercookies.ErrNoCookieFound) {
		return config.BrowserSession{}, false, nil
	}
	return config.BrowserSession{}, false, err
}
