package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janekbaraniewski/openusage/internal/core"
)

// browserSessionFakeServices augments fakeServices with browser-session-flow
// hooks so we can assert which calls the TUI made into the daemon-side
// service. Defined alongside fakeServices in
// telemetry_mapping_input_test.go; this file extends behaviour for new
// tests via embedding.
type browserSessionFakeServices struct {
	*fakeServices

	connectAccountID  string
	connectDomain     string
	connectCookieName string
	connectPreferred  string
	connectInfo       core.BrowserSessionInfo
	connectErr        error

	disconnectedAccountID string
	disconnectErr         error

	openedURL string
	openErr   error

	loadInfo core.BrowserSessionInfo
}

func (b *browserSessionFakeServices) ConnectBrowserSession(accountID, domain, cookieName, preferred string) (core.BrowserSessionInfo, error) {
	b.connectAccountID = accountID
	b.connectDomain = domain
	b.connectCookieName = cookieName
	b.connectPreferred = preferred
	if b.connectErr != nil {
		return core.BrowserSessionInfo{}, b.connectErr
	}
	return b.connectInfo, nil
}

func (b *browserSessionFakeServices) DisconnectBrowserSession(accountID string) error {
	b.disconnectedAccountID = accountID
	return b.disconnectErr
}

func (b *browserSessionFakeServices) LoadBrowserSessionInfo(string) core.BrowserSessionInfo {
	return b.loadInfo
}

func (b *browserSessionFakeServices) OpenProviderConsole(url string) error {
	b.openedURL = url
	return b.openErr
}

func newBrowserSessionFake() *browserSessionFakeServices {
	return &browserSessionFakeServices{fakeServices: &fakeServices{}}
}

// connectBrowserSessionCmd → on success the message carries info.
func TestConnectBrowserSessionCmd_Success(t *testing.T) {
	fake := newBrowserSessionFake()
	fake.connectInfo = core.BrowserSessionInfo{
		Connected:     true,
		Domain:        ".opencode.ai",
		CookieName:    "auth",
		SourceBrowser: "chrome",
	}
	m := Model{services: fake}

	cmd := m.connectBrowserSessionCmd("opencode-console", ".opencode.ai", "auth", "")
	msg := cmd().(browserSessionConnectedMsg)

	if msg.Err != nil {
		t.Fatalf("Err = %v, want nil", msg.Err)
	}
	if msg.AccountID != "opencode-console" {
		t.Errorf("AccountID = %q, want opencode-console", msg.AccountID)
	}
	if !msg.Info.Connected {
		t.Error("Info.Connected = false, want true")
	}
	if msg.Info.SourceBrowser != "chrome" {
		t.Errorf("SourceBrowser = %q, want chrome", msg.Info.SourceBrowser)
	}
	if fake.connectDomain != ".opencode.ai" || fake.connectCookieName != "auth" {
		t.Errorf("service called with wrong args: domain=%q cookie=%q", fake.connectDomain, fake.connectCookieName)
	}
}

// connectBrowserSessionCmd → propagates errors as msg.Err.
func TestConnectBrowserSessionCmd_FailurePropagated(t *testing.T) {
	fake := newBrowserSessionFake()
	fake.connectErr = errors.New("no cookie in any browser")
	m := Model{services: fake}

	cmd := m.connectBrowserSessionCmd("opencode-console", ".opencode.ai", "auth", "")
	msg := cmd().(browserSessionConnectedMsg)

	if msg.Err == nil || msg.Err.Error() != "no cookie in any browser" {
		t.Errorf("Err = %v, want 'no cookie in any browser'", msg.Err)
	}
}

// connectBrowserSessionCmd with nil services → returns error message rather
// than panicking. Daemon-disconnect path.
func TestConnectBrowserSessionCmd_NoServices(t *testing.T) {
	m := Model{services: nil}
	cmd := m.connectBrowserSessionCmd("x", ".x.com", "auth", "")
	msg := cmd().(browserSessionConnectedMsg)
	if msg.Err == nil {
		t.Fatal("expected error, got nil")
	}
}

// disconnectBrowserSessionCmd → calls service, propagates account ID.
func TestDisconnectBrowserSessionCmd(t *testing.T) {
	fake := newBrowserSessionFake()
	m := Model{services: fake}

	cmd := m.disconnectBrowserSessionCmd("opencode-console")
	msg := cmd().(browserSessionDisconnectedMsg)

	if msg.AccountID != "opencode-console" {
		t.Errorf("AccountID = %q", msg.AccountID)
	}
	if fake.disconnectedAccountID != "opencode-console" {
		t.Errorf("service not called with correct account: %q", fake.disconnectedAccountID)
	}
	if msg.Err != nil {
		t.Errorf("Err = %v, want nil", msg.Err)
	}
}

// openProviderConsoleCmd → invokes service with URL and propagates errors.
func TestOpenProviderConsoleCmd(t *testing.T) {
	fake := newBrowserSessionFake()
	m := Model{services: fake}

	cmd := m.openProviderConsoleCmd("https://opencode.ai/login")
	msg := cmd().(providerConsoleOpenedMsg)

	if fake.openedURL != "https://opencode.ai/login" {
		t.Errorf("openedURL = %q", fake.openedURL)
	}
	if msg.URL != "https://opencode.ai/login" {
		t.Errorf("msg.URL = %q", msg.URL)
	}
	if msg.Err != nil {
		t.Errorf("Err = %v", msg.Err)
	}
}

func apiKeyTabRowIndex(t *testing.T, m Model, accountID string) int {
	t.Helper()
	ids := m.apiKeysTabIDs()
	for i, id := range ids {
		if id == accountID {
			return i
		}
	}
	t.Fatalf("account %q not found in apiKeysTabIDs: %v", accountID, ids)
	return -1
}

func TestHandleSettingsModalKey_PrimaryBrowserSessionEnterStartsPicker(t *testing.T) {
	m := Model{services: newBrowserSessionFake()}
	m.settings.tab = settingsTabAPIKeys
	m.settings.cursor = apiKeyTabRowIndex(t, m, "perplexity")

	updated, cmd := m.handleSettingsModalKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	if !got.settings.browserPicker.active {
		t.Fatal("browser picker not activated for primary browser-session provider")
	}
	if got.settings.browserPicker.accountID != "perplexity" {
		t.Fatalf("picker accountID = %q, want perplexity", got.settings.browserPicker.accountID)
	}
	if cmd == nil {
		t.Fatal("expected browser enumeration command")
	}
}

func TestHandleSettingsModalKey_SupplementalBrowserSessionEnterEditsAPIKey(t *testing.T) {
	m := Model{}
	m.settings.tab = settingsTabAPIKeys
	m.providerOrder = []string{"opencode"}
	m.accountProviders = map[string]string{"opencode": "opencode"}

	updated, cmd := m.handleSettingsModalKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	if !got.settings.apiKeyEditing {
		t.Fatal("Enter should edit API key for mixed-auth providers")
	}
	if got.settings.browserPicker.active {
		t.Fatal("browser picker should stay inactive on Enter for mixed-auth providers")
	}
	if cmd != nil {
		t.Fatalf("unexpected command on Enter: %T", cmd())
	}
}

func TestHandleSettingsModalKey_SupplementalBrowserSessionConnectUsesC(t *testing.T) {
	m := Model{services: newBrowserSessionFake()}
	m.settings.tab = settingsTabAPIKeys
	m.providerOrder = []string{"opencode"}
	m.accountProviders = map[string]string{"opencode": "opencode"}

	updated, cmd := m.handleSettingsModalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	got := updated.(Model)

	if !got.settings.browserPicker.active {
		t.Fatal("browser picker not activated for supplemental browser-session connect")
	}
	if cmd == nil {
		t.Fatal("expected browser enumeration command")
	}
}

func TestHandleSettingsModalKey_SupplementalBrowserSessionRequiresPrimaryCredential(t *testing.T) {
	t.Setenv("OPENCODE_API_KEY", "")
	t.Setenv("ZEN_API_KEY", "")

	m := Model{services: newBrowserSessionFake()}
	m.settings.tab = settingsTabAPIKeys
	m.settings.cursor = apiKeyTabRowIndex(t, m, "opencode")

	updated, cmd := m.handleSettingsModalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	got := updated.(Model)

	if got.settings.browserPicker.active {
		t.Fatal("browser picker should not open without primary credential")
	}
	if cmd != nil {
		t.Fatalf("unexpected command without primary credential: %T", cmd())
	}
	if got.settings.apiKeyStatus != "configure the API key first, then connect browser session" {
		t.Fatalf("apiKeyStatus = %q", got.settings.apiKeyStatus)
	}
}

func TestHandleSettingsModalKey_SupplementalBrowserSessionAliasEnvAllowsConnect(t *testing.T) {
	t.Setenv("OPENCODE_API_KEY", "")
	t.Setenv("ZEN_API_KEY", "zen-test-key")

	m := Model{services: newBrowserSessionFake()}
	m.settings.tab = settingsTabAPIKeys
	m.settings.cursor = apiKeyTabRowIndex(t, m, "opencode")

	updated, cmd := m.handleSettingsModalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	got := updated.(Model)

	if !got.settings.browserPicker.active {
		t.Fatal("browser picker should open when alias API key env is configured")
	}
	if cmd == nil {
		t.Fatal("expected browser enumeration command")
	}
}

func TestBrowserSessionConnectedMsg_PrimaryProviderRegistersAccount(t *testing.T) {
	var captured core.AccountConfig
	m := Model{}
	m.onAddAccount = func(acct core.AccountConfig) { captured = acct }

	updated, _ := m.Update(browserSessionConnectedMsg{
		AccountID: "perplexity",
		Info: core.BrowserSessionInfo{
			Connected:     true,
			Domain:        ".perplexity.ai",
			CookieName:    "__Secure-next-auth.session-token",
			SourceBrowser: "chrome",
		},
	})
	got := updated.(Model)

	if captured.Provider != "perplexity" {
		t.Fatalf("captured provider = %q, want perplexity", captured.Provider)
	}
	if captured.Auth != "browser_session" {
		t.Fatalf("captured auth = %q, want browser_session", captured.Auth)
	}
	if captured.BrowserCookie == nil || captured.BrowserCookie.SourceBrowser != "chrome" {
		t.Fatalf("captured browser cookie = %+v, want source browser chrome", captured.BrowserCookie)
	}
	if got.accountProviders["perplexity"] != "perplexity" {
		t.Fatalf("accountProviders[perplexity] = %q", got.accountProviders["perplexity"])
	}
}

func TestBrowserSessionConnectedMsg_MixedAuthKeepsAPIKeyPrimary(t *testing.T) {
	var captured core.AccountConfig
	m := Model{
		accountProviders: map[string]string{"opencode": "opencode"},
		providerOrder:    []string{"opencode"},
	}
	m.onAddAccount = func(acct core.AccountConfig) { captured = acct }

	updated, _ := m.Update(browserSessionConnectedMsg{
		AccountID: "opencode",
		Info: core.BrowserSessionInfo{
			Connected:     true,
			Domain:        ".opencode.ai",
			CookieName:    "auth",
			SourceBrowser: "firefox",
		},
	})
	got := updated.(Model)

	if captured.Provider != "opencode" {
		t.Fatalf("captured provider = %q, want opencode", captured.Provider)
	}
	if captured.Auth != "api_key" {
		t.Fatalf("captured auth = %q, want api_key", captured.Auth)
	}
	if captured.APIKeyEnv != "OPENCODE_API_KEY" {
		t.Fatalf("captured APIKeyEnv = %q, want OPENCODE_API_KEY", captured.APIKeyEnv)
	}
	if captured.BrowserCookie == nil || captured.BrowserCookie.SourceBrowser != "firefox" {
		t.Fatalf("captured browser cookie = %+v, want source browser firefox", captured.BrowserCookie)
	}
	if got.accountProviders["opencode"] != "opencode" {
		t.Fatalf("accountProviders[opencode] = %q", got.accountProviders["opencode"])
	}
}

func TestBrowserSessionConnectedMsg_MixedAuthUsesConfiguredAliasEnv(t *testing.T) {
	t.Setenv("OPENCODE_API_KEY", "")
	t.Setenv("ZEN_API_KEY", "zen-test-key")

	var captured core.AccountConfig
	m := Model{}
	m.onAddAccount = func(acct core.AccountConfig) { captured = acct }

	updated, _ := m.Update(browserSessionConnectedMsg{
		AccountID: "opencode",
		Info: core.BrowserSessionInfo{
			Connected:     true,
			Domain:        ".opencode.ai",
			CookieName:    "auth",
			SourceBrowser: "firefox",
		},
	})
	got := updated.(Model)

	if captured.Provider != "opencode" {
		t.Fatalf("captured provider = %q, want opencode", captured.Provider)
	}
	if captured.Auth != "api_key" {
		t.Fatalf("captured auth = %q, want api_key", captured.Auth)
	}
	if captured.APIKeyEnv != "ZEN_API_KEY" {
		t.Fatalf("captured APIKeyEnv = %q, want ZEN_API_KEY", captured.APIKeyEnv)
	}
	if got.accountProviders["opencode"] != "opencode" {
		t.Fatalf("accountProviders[opencode] = %q", got.accountProviders["opencode"])
	}
}
