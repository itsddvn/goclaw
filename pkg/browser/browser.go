package browser

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Manager handles the Chrome browser lifecycle and page management.
type Manager struct {
	mu       sync.Mutex
	browser  *rod.Browser
	refs     *RefStore
	pages    map[string]*rod.Page        // targetID → page
	console  map[string][]ConsoleMessage // targetID → console messages
	headless bool
	logger   *slog.Logger
}

// Option configures a Manager.
type Option func(*Manager)

// WithHeadless sets headless mode (default false).
func WithHeadless(h bool) Option {
	return func(m *Manager) { m.headless = h }
}

// WithLogger sets a custom logger.
func WithLogger(l *slog.Logger) Option {
	return func(m *Manager) { m.logger = l }
}

// New creates a Manager with options.
func New(opts ...Option) *Manager {
	m := &Manager{
		refs:    NewRefStore(),
		pages:   make(map[string]*rod.Page),
		console: make(map[string][]ConsoleMessage),
		logger:  slog.Default(),
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// Start launches a Chrome browser.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.browser != nil {
		return fmt.Errorf("browser already running")
	}

	l := launcher.New().
		Headless(m.headless).
		Set("disable-gpu").
		Set("no-first-run").
		Set("no-default-browser-check")

	controlURL, err := l.Launch()
	if err != nil {
		return fmt.Errorf("launch Chrome: %w", err)
	}

	m.logger.Info("Chrome launched", "cdp", controlURL, "headless", m.headless)

	b := rod.New().ControlURL(controlURL)
	if err := b.Connect(); err != nil {
		return fmt.Errorf("connect to Chrome: %w", err)
	}

	m.browser = b
	return nil
}

// Stop closes the Chrome browser.
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.browser == nil {
		return nil
	}

	err := m.browser.Close()
	m.browser = nil
	m.pages = make(map[string]*rod.Page)
	m.console = make(map[string][]ConsoleMessage)
	return err
}

// Status returns current browser status.
func (m *Manager) Status() *StatusInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.browser == nil {
		return &StatusInfo{Running: false}
	}

	pages, _ := m.browser.Pages()
	info := &StatusInfo{
		Running: true,
		Tabs:    len(pages),
	}
	if len(pages) > 0 {
		if pageInfo, err := pages[0].Info(); err == nil {
			info.URL = pageInfo.URL
		}
	}
	return info
}

// ListTabs returns all open tabs.
func (m *Manager) ListTabs(ctx context.Context) ([]TabInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.browser == nil {
		return nil, fmt.Errorf("browser not running")
	}

	pages, err := m.browser.Pages()
	if err != nil {
		return nil, fmt.Errorf("list pages: %w", err)
	}

	tabs := make([]TabInfo, 0, len(pages))
	for _, p := range pages {
		info, err := p.Info()
		if err != nil || info == nil {
			continue
		}
		tid := string(p.TargetID)
		m.pages[tid] = p
		tabs = append(tabs, TabInfo{
			TargetID: tid,
			URL:      info.URL,
			Title:    info.Title,
		})
	}
	return tabs, nil
}

// OpenTab opens a new tab with the given URL.
func (m *Manager) OpenTab(ctx context.Context, url string) (*TabInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.browser == nil {
		return nil, fmt.Errorf("browser not running")
	}

	page, err := m.browser.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return nil, fmt.Errorf("open tab: %w", err)
	}

	if err := page.WaitStable(300 * time.Millisecond); err != nil {
		return nil, fmt.Errorf("wait stable: %w", err)
	}
	info, _ := page.Info()
	tid := string(page.TargetID)
	m.pages[tid] = page

	// Set up console listener
	m.setupConsoleListener(page, tid)

	tab := &TabInfo{TargetID: tid, URL: url}
	if info != nil {
		tab.URL = info.URL
		tab.Title = info.Title
	}
	return tab, nil
}

// FocusTab activates a tab.
func (m *Manager) FocusTab(ctx context.Context, targetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	page, err := m.getPage(targetID)
	if err != nil {
		return err
	}

	_, err = page.Activate()
	return err
}

// CloseTab closes a tab.
func (m *Manager) CloseTab(ctx context.Context, targetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	page, err := m.getPage(targetID)
	if err != nil {
		return err
	}

	delete(m.pages, targetID)
	delete(m.console, targetID)
	return page.Close()
}

// ConsoleMessages returns captured console messages for a tab.
func (m *Manager) ConsoleMessages(targetID string) []ConsoleMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	msgs := m.console[targetID]
	if msgs == nil {
		return []ConsoleMessage{}
	}

	// Return copy and clear
	result := make([]ConsoleMessage, len(msgs))
	copy(result, msgs)
	m.console[targetID] = nil
	return result
}

// Snapshot takes an accessibility snapshot of a page.
func (m *Manager) Snapshot(ctx context.Context, targetID string, opts SnapshotOptions) (*SnapshotResult, error) {
	m.mu.Lock()
	page, err := m.getPage(targetID)
	m.mu.Unlock()

	if err != nil {
		return nil, err
	}

	result, err := proto.AccessibilityGetFullAXTree{}.Call(page)
	if err != nil {
		return nil, fmt.Errorf("get AX tree: %w", err)
	}

	snap := FormatSnapshot(result.Nodes, opts)
	info, _ := page.Info()
	snap.TargetID = targetID
	if info != nil {
		snap.URL = info.URL
		snap.Title = info.Title
	}

	// Cache refs
	m.refs.Store(targetID, snap.Refs)

	return snap, nil
}

// Screenshot captures a page screenshot as PNG bytes.
func (m *Manager) Screenshot(ctx context.Context, targetID string, fullPage bool) ([]byte, error) {
	m.mu.Lock()
	page, err := m.getPage(targetID)
	m.mu.Unlock()

	if err != nil {
		return nil, err
	}

	if fullPage {
		return page.Screenshot(fullPage, &proto.PageCaptureScreenshot{
			Format: proto.PageCaptureScreenshotFormatPng,
		})
	}
	return page.Screenshot(false, nil)
}

// Navigate navigates a page to a URL.
func (m *Manager) Navigate(ctx context.Context, targetID, url string) error {
	m.mu.Lock()
	page, err := m.getPage(targetID)
	m.mu.Unlock()

	if err != nil {
		return err
	}

	if err := page.Navigate(url); err != nil {
		return fmt.Errorf("navigate: %w", err)
	}
	if err := page.WaitStable(300 * time.Millisecond); err != nil {
		return fmt.Errorf("wait stable after navigate: %w", err)
	}
	return nil
}

// Close shuts down the browser if running.
func (m *Manager) Close() error {
	return m.Stop(context.Background())
}

// Refs returns the RefStore for external use (e.g. actions).
func (m *Manager) Refs() *RefStore {
	return m.refs
}

// getPage looks up a page by targetID. If targetID is empty, returns the first available page.
// Must be called with m.mu held.
func (m *Manager) getPage(targetID string) (*rod.Page, error) {
	if m.browser == nil {
		return nil, fmt.Errorf("browser not running")
	}

	// If targetID specified, look in cache first
	if targetID != "" {
		if p, ok := m.pages[targetID]; ok {
			return p, nil
		}
	}

	// Refresh page list from browser
	pages, err := m.browser.Pages()
	if err != nil {
		return nil, fmt.Errorf("list pages: %w", err)
	}

	// Update cache
	for _, p := range pages {
		tid := string(p.TargetID)
		m.pages[tid] = p
	}

	if targetID != "" {
		if p, ok := m.pages[targetID]; ok {
			return p, nil
		}
		return nil, fmt.Errorf("tab not found: %s", targetID)
	}

	// No targetID: return first page
	if len(pages) == 0 {
		return nil, fmt.Errorf("no tabs open")
	}
	return pages[0], nil
}

// setupConsoleListener attaches a console message listener to a page via Rod's EachEvent.
func (m *Manager) setupConsoleListener(page *rod.Page, targetID string) {
	go page.EachEvent(func(e *proto.RuntimeConsoleAPICalled) {
		var text string
		for _, arg := range e.Args {
			s := arg.Value.String()
			if s != "" && s != "null" {
				text += s + " "
			}
		}

		level := "log"
		switch e.Type {
		case proto.RuntimeConsoleAPICalledTypeWarning:
			level = "warn"
		case proto.RuntimeConsoleAPICalledTypeError:
			level = "error"
		case proto.RuntimeConsoleAPICalledTypeInfo:
			level = "info"
		}

		m.mu.Lock()
		msgs := m.console[targetID]
		if len(msgs) >= 500 {
			msgs = msgs[1:]
		}
		m.console[targetID] = append(msgs, ConsoleMessage{
			Level: level,
			Text:  text,
		})
		m.mu.Unlock()
	})()
}

// resolveElement converts a RoleRef to a Rod Element via backendNodeID.
func (m *Manager) resolveElement(page *rod.Page, targetID, ref string) (*rod.Element, error) {
	roleRef, ok := m.refs.Resolve(targetID, ref)
	if !ok {
		return nil, fmt.Errorf("unknown ref %q — take a new snapshot first", ref)
	}

	if roleRef.BackendNodeID == 0 {
		return nil, fmt.Errorf("no backendNodeID for ref %q", ref)
	}

	backendID := proto.DOMBackendNodeID(roleRef.BackendNodeID)
	resolved, err := proto.DOMResolveNode{BackendNodeID: backendID}.Call(page)
	if err != nil {
		return nil, fmt.Errorf("resolve DOM node for %q (backendNodeID=%d): %w", ref, roleRef.BackendNodeID, err)
	}

	el, err := page.ElementFromObject(resolved.Object)
	if err != nil {
		return nil, fmt.Errorf("get element from object for %q: %w", ref, err)
	}

	return el, nil
}

// getPageAndResolve is a helper that locks, gets page, and resolves an element.
func (m *Manager) getPageAndResolve(targetID, ref string) (*rod.Page, *rod.Element, error) {
	m.mu.Lock()
	page, err := m.getPage(targetID)
	m.mu.Unlock()
	if err != nil {
		return nil, nil, err
	}

	// Ensure DOM is enabled for node resolution
	_ = proto.DOMEnable{}.Call(page)

	el, err := m.resolveElement(page, targetID, NormalizeRef(ref))
	if err != nil {
		return nil, nil, err
	}

	return page, el, nil
}

// waitStable waits for page to become stable (no network/DOM activity).
func waitStable(page *rod.Page) {
	_ = page.WaitStable(300 * time.Millisecond)
}
