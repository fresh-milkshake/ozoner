package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/chromedp/chromedp"
)

// BrowserClient handles headless browser operations
type BrowserClient struct {
	allocCtx context.Context
	ctx      context.Context
	cancel   context.CancelFunc
	timeout  time.Duration
}

// BrowserClientConfig holds configuration for BrowserClient
type BrowserClientConfig struct {
	Timeout      time.Duration
	Headless     bool
	UserAgent    string
	WindowWidth  int
	WindowHeight int
}

// DefaultBrowserClientConfig returns default configuration
func DefaultBrowserClientConfig() BrowserClientConfig {
	return BrowserClientConfig{
		Timeout:      60 * time.Second,
		Headless:     true,
		UserAgent:    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		WindowWidth:  1920,
		WindowHeight: 1080,
	}
}

// NewBrowserClient creates a new browser client
func NewBrowserClient(config BrowserClientConfig) (*BrowserClient, error) {
	log.Info("Initializing browser client", "headless", config.Headless, "timeout", config.Timeout)

	// Enhanced Chrome options for anti-bot evasion
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", config.Headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-sandbox", true),
		// Anti-bot evasion flags
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("exclude-switches", "enable-automation"),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("disable-extensions", false), // Keep extensions to look more natural
		chromedp.Flag("disable-plugins-discovery", false),
		chromedp.Flag("disable-default-apps", false),
		// WebDriver detection evasion
		chromedp.Flag("disable-web-security", false),
		chromedp.Flag("disable-features", "IsolateOrigins,site-per-process"),
		// Performance and stability
		chromedp.Flag("disable-background-networking", false),
		chromedp.Flag("disable-background-timer-throttling", false),
		chromedp.Flag("disable-renderer-backgrounding", false),
		chromedp.Flag("disable-backgrounding-occluded-windows", false),
		chromedp.Flag("disable-ipc-flooding-protection", false),
		// User agent and window
		chromedp.UserAgent(config.UserAgent),
		chromedp.WindowSize(config.WindowWidth, config.WindowHeight),
	)

	log.Debug("Creating browser allocator", "options_count", len(opts))
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancelCtx := chromedp.NewContext(allocCtx,
		chromedp.WithLogf(func(format string, v ...interface{}) {
			// Log chromedp messages at debug level
			log.Debug(fmt.Sprintf(format, v...))
		}),
	)

	client := &BrowserClient{
		allocCtx: allocCtx,
		ctx:      ctx,
		cancel:   func() { cancelCtx(); cancelAlloc() },
		timeout:  config.Timeout,
	}

	log.Debug("Starting browser", "ctx_valid", ctx.Err() == nil)
	// In chromedp, the browser is initialized lazily on first chromedp.Run call
	// We don't need to initialize it here - it will be initialized on first navigation
	// Just verify the context is valid

	// Check context is valid
	select {
	case <-ctx.Done():
		log.Error("Browser context already canceled", "ctx_err", ctx.Err())
		client.Close()
		return nil, fmt.Errorf("browser context canceled: %w", ctx.Err())
	default:
		log.Debug("Browser context is valid, browser will be initialized on first use")
	}

	log.Info("Browser client initialized successfully")
	return client, nil
}

// getContext creates a context for browser operations
// It uses the browser's internal context with timeout, but does NOT cancel the browser context
// when parent context is canceled. This allows the browser to be reused across multiple operations.
// The parent context is checked manually in operations to respect cancellation requests.
func (c *BrowserClient) getContext(parentCtx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	// Check if browser context is still valid
	select {
	case <-c.ctx.Done():
		log.Error("Browser context already canceled, cannot proceed", "ctx_err", c.ctx.Err())
		// Return canceled context
		ctx, cancel := context.WithCancel(parentCtx)
		cancel()
		return ctx, cancel
	default:
	}

	// Check if parent context is already canceled
	select {
	case <-parentCtx.Done():
		log.Debug("Parent context already canceled", "ctx_err", parentCtx.Err())
		ctx, cancel := context.WithCancel(parentCtx)
		return ctx, cancel
	default:
	}

	// Use browser context as base with timeout
	// IMPORTANT: We do NOT link this to parent context cancellation
	// The browser context should remain alive even if parent context is canceled
	// This allows the browser to be reused for subsequent operations
	browserTimeoutCtx, browserCancel := context.WithTimeout(c.ctx, timeout)

	cancelFunc := func() {
		// Only cancel the timeout context, not the browser context
		browserCancel()
	}

	return browserTimeoutCtx, cancelFunc
}

// Navigate navigates to a URL and waits for page load
func (c *BrowserClient) Navigate(ctx context.Context, url string) error {
	log.Info("Navigating to URL", "url", url)

	// Check if parent context is already canceled
	select {
	case <-ctx.Done():
		return fmt.Errorf("parent context canceled: %w", ctx.Err())
	default:
	}

	// Check browser context first with detailed logging
	select {
	case <-c.ctx.Done():
		log.Error("Browser context is canceled before navigation", "ctx_err", c.ctx.Err())
		return fmt.Errorf("browser context is canceled, cannot navigate")
	default:
		log.Debug("Browser context is valid, proceeding with navigation")
	}

	// Use browser context directly - no child contexts to avoid cancellation issues
	startTime := time.Now()
	deadline := time.Now().Add(c.timeout)
	log.Debug("Executing navigation commands", "url", url)

	// Small delay before navigation (reduced for performance)
	preDelay := time.Duration(100+rand.Intn(200)) * time.Millisecond
	time.Sleep(preDelay)

	// Anti-bot evasion script to inject on first page load
	evasionScript := `
		Object.defineProperty(navigator, 'webdriver', {
			get: () => undefined
		});
		
		// Override plugins
		Object.defineProperty(navigator, 'plugins', {
			get: () => [1, 2, 3, 4, 5]
		});
		
		// Override languages
		Object.defineProperty(navigator, 'languages', {
			get: () => ['ru-RU', 'ru', 'en-US', 'en']
		});
		
		// Override permissions
		const originalQuery = window.navigator.permissions.query;
		window.navigator.permissions.query = (parameters) => (
			parameters.name === 'notifications' ?
				Promise.resolve({ state: Notification.permission }) :
				originalQuery(parameters)
		);
		
		// Chrome runtime
		window.chrome = {
			runtime: {}
		};
	`

	// Run navigation using browser context directly with manual parent context and timeout checks
	err := chromedp.Run(c.ctx,
		// Check parent context and timeout before starting
		chromedp.ActionFunc(func(actCtx context.Context) error {
			select {
			case <-ctx.Done():
				return fmt.Errorf("parent context canceled: %w", ctx.Err())
			default:
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("navigation timeout")
			}
			return nil
		}),
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		// Wait for page to be interactive
		chromedp.WaitReady("body", chromedp.ByQuery),
		// Check parent context and timeout during wait
		chromedp.ActionFunc(func(actCtx context.Context) error {
			select {
			case <-ctx.Done():
				return fmt.Errorf("parent context canceled: %w", ctx.Err())
			default:
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("navigation timeout")
			}
			return nil
		}),
		// Inject anti-bot evasion script
		chromedp.Evaluate(evasionScript, nil),
		// Reduced delay to mimic human behavior (optimized)
		chromedp.Sleep(time.Duration(300+rand.Intn(400))*time.Millisecond),
		// Check for anti-bot challenges with reduced wait
		chromedp.ActionFunc(func(actCtx context.Context) error {
			// Check parent context and timeout
			select {
			case <-ctx.Done():
				return fmt.Errorf("parent context canceled: %w", ctx.Err())
			default:
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("navigation timeout")
			}
			// Reduced wait time
			time.Sleep(500 * time.Millisecond)
			return nil
		}),
	)
	duration := time.Since(startTime)

	if err != nil {
		log.Error("Navigation failed", "err", err, "url", url, "duration_ms", duration.Milliseconds())
		// Check if it's a context cancellation
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("navigation timeout or canceled: %w", err)
		}
		return err
	}

	// Check parent context after navigation
	select {
	case <-ctx.Done():
		return fmt.Errorf("parent context canceled after navigation: %w", ctx.Err())
	default:
	}

	log.Info("Navigation successful", "url", url, "duration_ms", duration.Milliseconds())
	return nil
}

// GetHTML returns the HTML content of the current page
func (c *BrowserClient) GetHTML(ctx context.Context) (string, error) {
	log.Debug("Extracting HTML from page")

	// Check if parent context is already canceled
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("parent context canceled: %w", ctx.Err())
	default:
	}

	// Check browser context
	select {
	case <-c.ctx.Done():
		return "", fmt.Errorf("browser context canceled: %w", c.ctx.Err())
	default:
	}

	var html string
	// Use browser context directly - no child contexts to avoid cancellation issues
	deadline := time.Now().Add(c.timeout)

	startTime := time.Now()
	err := chromedp.Run(c.ctx,
		// Check parent context and timeout before starting
		chromedp.ActionFunc(func(actCtx context.Context) error {
			select {
			case <-ctx.Done():
				return fmt.Errorf("parent context canceled: %w", ctx.Err())
			default:
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("operation timeout")
			}
			return nil
		}),
		// Wait for content to be ready
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
	)
	duration := time.Since(startTime)

	if err != nil {
		log.Error("Failed to extract HTML", "err", err, "duration_ms", duration.Milliseconds())
		return "", err
	}

	log.Info("HTML extracted successfully", "size_bytes", len(html), "duration_ms", duration.Milliseconds())
	return html, nil
}

// GetText returns text content of an element
func (c *BrowserClient) GetText(ctx context.Context, selector string) (string, error) {
	// Check browser context
	select {
	case <-c.ctx.Done():
		return "", fmt.Errorf("browser context canceled: %w", c.ctx.Err())
	default:
	}

	// Check parent context
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("parent context canceled: %w", ctx.Err())
	default:
	}

	// Use browser context directly - no child contexts to avoid cancellation issues
	deadline := time.Now().Add(c.timeout)

	var text string
	err := chromedp.Run(c.ctx,
		chromedp.ActionFunc(func(actCtx context.Context) error {
			select {
			case <-ctx.Done():
				return fmt.Errorf("parent context canceled: %w", ctx.Err())
			default:
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("operation timeout")
			}
			return nil
		}),
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Text(selector, &text, chromedp.ByQuery),
	)
	return text, err
}

// WaitForSelector waits for an element to appear
func (c *BrowserClient) WaitForSelector(ctx context.Context, selector string, timeout time.Duration) error {
	// Check browser context
	select {
	case <-c.ctx.Done():
		return fmt.Errorf("browser context canceled: %w", c.ctx.Err())
	default:
	}

	// Check parent context
	select {
	case <-ctx.Done():
		return fmt.Errorf("parent context canceled: %w", ctx.Err())
	default:
	}

	// Use browser context directly - no child contexts to avoid cancellation issues
	deadline := time.Now().Add(timeout)

	return chromedp.Run(c.ctx,
		chromedp.ActionFunc(func(actCtx context.Context) error {
			select {
			case <-ctx.Done():
				return fmt.Errorf("parent context canceled: %w", ctx.Err())
			default:
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("operation timeout")
			}
			return nil
		}),
		chromedp.WaitVisible(selector, chromedp.ByQuery),
	)
}

// Evaluate runs JavaScript and returns the result
func (c *BrowserClient) Evaluate(ctx context.Context, script string, result interface{}) error {
	// Check browser context
	select {
	case <-c.ctx.Done():
		return fmt.Errorf("browser context canceled: %w", c.ctx.Err())
	default:
	}

	// Check parent context
	select {
	case <-ctx.Done():
		return fmt.Errorf("parent context canceled: %w", ctx.Err())
	default:
	}

	// Use browser context directly - no child contexts to avoid cancellation issues
	deadline := time.Now().Add(c.timeout)

	return chromedp.Run(c.ctx,
		chromedp.ActionFunc(func(actCtx context.Context) error {
			select {
			case <-ctx.Done():
				return fmt.Errorf("parent context canceled: %w", ctx.Err())
			default:
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("operation timeout")
			}
			return nil
		}),
		chromedp.Evaluate(script, result),
	)
}

// ScrollToBottom scrolls the page to bottom (useful for lazy loading)
func (c *BrowserClient) ScrollToBottom(ctx context.Context) error {
	// Check browser context
	select {
	case <-c.ctx.Done():
		return fmt.Errorf("browser context canceled: %w", c.ctx.Err())
	default:
	}

	// Check parent context
	select {
	case <-ctx.Done():
		return fmt.Errorf("parent context canceled: %w", ctx.Err())
	default:
	}

	// Use browser context directly - no child contexts to avoid cancellation issues
	deadline := time.Now().Add(c.timeout)

	// Human-like scrolling with pauses
	script := `
		(function() {
			return new Promise((resolve) => {
				let totalHeight = document.body.scrollHeight;
				let currentPosition = window.pageYOffset || document.documentElement.scrollTop;
				let distance = totalHeight - currentPosition;
				let scrollStep = Math.min(300, distance / 5);
				let scrollInterval = setInterval(() => {
					currentPosition += scrollStep;
					window.scrollTo(0, currentPosition);
					
					if (currentPosition >= totalHeight - 100) {
						clearInterval(scrollInterval);
						setTimeout(resolve, 500);
					}
				}, 50);
			});
		})();
	`

	return chromedp.Run(c.ctx,
		chromedp.ActionFunc(func(actCtx context.Context) error {
			select {
			case <-ctx.Done():
				return fmt.Errorf("parent context canceled: %w", ctx.Err())
			default:
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("operation timeout")
			}
			return nil
		}),
		chromedp.Evaluate(script, nil),
	)
}

// HumanLikeScroll performs human-like scrolling with random pauses
func (c *BrowserClient) HumanLikeScroll(ctx context.Context) error {
	// Check if parent context is already canceled
	select {
	case <-ctx.Done():
		return fmt.Errorf("parent context canceled: %w", ctx.Err())
	default:
	}

	// Check browser context
	select {
	case <-c.ctx.Done():
		return fmt.Errorf("browser context canceled: %w", c.ctx.Err())
	default:
	}

	// Use browser context directly - no child contexts to avoid cancellation issues
	deadline := time.Now().Add(c.timeout)

	script := `
		(function() {
			return new Promise((resolve) => {
				let scrollAmount = 0;
				let maxScroll = document.body.scrollHeight - window.innerHeight;
				let scrollStep = 200 + Math.random() * 300;
				let pauseChance = 0.3;
				
				function scroll() {
					if (scrollAmount >= maxScroll) {
						setTimeout(resolve, 500);
						return;
					}
					
					scrollAmount += scrollStep;
					window.scrollTo({
						top: scrollAmount,
						behavior: 'smooth'
					});
					
					let delay = 100 + Math.random() * 200;
					if (Math.random() < pauseChance) {
						delay += 300 + Math.random() * 500;
					}
					
					setTimeout(scroll, delay);
				}
				
				scroll();
			});
		})();
	`

	return chromedp.Run(c.ctx,
		chromedp.ActionFunc(func(actCtx context.Context) error {
			select {
			case <-ctx.Done():
				return fmt.Errorf("parent context canceled: %w", ctx.Err())
			default:
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("operation timeout")
			}
			return nil
		}),
		chromedp.Evaluate(script, nil),
	)
}

// ExtractJSONData extracts JSON data from page (common pattern on Ozon)
func (c *BrowserClient) ExtractJSONData(ctx context.Context, script string) (string, error) {
	log.Debug("Extracting JSON data from page", "script_length", len(script))

	// Check if parent context is already canceled
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("parent context canceled: %w", ctx.Err())
	default:
	}

	// Check browser context
	select {
	case <-c.ctx.Done():
		return "", fmt.Errorf("browser context canceled: %w", c.ctx.Err())
	default:
	}

	var jsonData interface{}
	// Use browser context directly - no child contexts to avoid cancellation issues
	deadline := time.Now().Add(c.timeout)

	startTime := time.Now()
	err := chromedp.Run(c.ctx,
		// Check parent context and timeout before starting
		chromedp.ActionFunc(func(actCtx context.Context) error {
			select {
			case <-ctx.Done():
				return fmt.Errorf("parent context canceled: %w", ctx.Err())
			default:
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("operation timeout")
			}
			return nil
		}),
		// Wait for page to be ready
		chromedp.WaitReady("body", chromedp.ByQuery),
		// Reduced delay to ensure dynamic content is loaded
		chromedp.Sleep(300*time.Millisecond),
		chromedp.ActionFunc(func(actCtx context.Context) error {
			// Check parent context and timeout before evaluation
			select {
			case <-ctx.Done():
				return fmt.Errorf("parent context canceled: %w", ctx.Err())
			default:
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("operation timeout")
			}
			return nil
		}),
		chromedp.Evaluate(script, &jsonData),
	)
	duration := time.Since(startTime)

	if err != nil {
		log.Error("Failed to evaluate script", "err", err, "duration_ms", duration.Milliseconds())
		return "", fmt.Errorf("failed to evaluate script: %w", err)
	}

	if jsonData == nil {
		log.Warn("No data extracted from page")
		return "", fmt.Errorf("no data extracted")
	}

	// Convert to JSON string
	jsonBytes, err := json.Marshal(jsonData)
	if err != nil {
		log.Error("Failed to marshal JSON", "err", err)
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	result := string(jsonBytes)
	// Remove quotes if the result is a JSON string
	if len(result) > 2 && result[0] == '"' && result[len(result)-1] == '"' {
		result = result[1 : len(result)-1]
		// Unescape
		result = strings.ReplaceAll(result, `\"`, `"`)
		result = strings.ReplaceAll(result, `\\`, `\`)
	}

	log.Info("JSON data extracted successfully", "size_bytes", len(result), "duration_ms", duration.Milliseconds())
	return result, nil
}

// Close closes the browser
func (c *BrowserClient) Close() error {
	c.cancel()
	return nil
}
