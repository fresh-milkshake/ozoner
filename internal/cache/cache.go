package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"ozoner/internal/models"

	"github.com/charmbracelet/log"
)

// CacheConfig holds configuration for the cache
type CacheConfig struct {
	Enabled        bool
	Dir            string
	MaxSize        int64  // Maximum cache size in bytes
	MaxItems       int    // Maximum number of items (0 = unlimited)
	EvictionPolicy string // "lru", "fifo", "none"
	AutoSave       bool
	OnSave         func(product *models.Product) error
}

// DefaultCacheConfig returns default cache configuration
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Enabled:        true,
		Dir:            ".ozoner/cache",
		MaxSize:        100 * 1024 * 1024, // 100MB
		MaxItems:       0,                 // Unlimited
		EvictionPolicy: "lru",
		AutoSave:       true,
	}
}

// ProductMetadata holds metadata about a cached product
type ProductMetadata struct {
	File         string    `json:"file"`
	Size         int64     `json:"size"`
	CachedAt     time.Time `json:"cached_at"`
	Source       string    `json:"source"`
	LastAccessed time.Time `json:"last_accessed"`
}

// CacheIndex holds the cache index
type CacheIndex struct {
	Products   map[string]ProductMetadata `json:"products"`
	TotalSize  int64                      `json:"total_size"`
	TotalItems int                        `json:"total_items"`
	mu         sync.RWMutex
}

// Stats holds cache statistics
type Stats struct {
	TotalSize      int64     `json:"total_size"`
	TotalItems     int       `json:"total_items"`
	OldestItem     time.Time `json:"oldest_item"`
	NewestItem     time.Time `json:"newest_item"`
	MaxSize        int64     `json:"max_size"`
	MaxItems       int       `json:"max_items"`
	EvictionPolicy string    `json:"eviction_policy"`
}

// Cache manages product caching
type Cache struct {
	config CacheConfig
	index  *CacheIndex
	mu     sync.RWMutex
}

// NewCache creates a new cache instance
func NewCache(config CacheConfig) (*Cache, error) {
	if !config.Enabled {
		return nil, nil
	}

	cache := &Cache{
		config: config,
		index: &CacheIndex{
			Products: make(map[string]ProductMetadata),
		},
	}

	// Create cache directory
	if err := os.MkdirAll(config.Dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Create products subdirectory
	productsDir := filepath.Join(config.Dir, "products")
	if err := os.MkdirAll(productsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create products directory: %w", err)
	}

	// Load existing index
	if err := cache.loadIndex(); err != nil {
		log.Warn("Failed to load cache index, starting fresh", "err", err)
	}

	return cache, nil
}

// loadIndex loads the cache index from disk
func (c *Cache) loadIndex() error {
	indexPath := filepath.Join(c.config.Dir, "index.json")

	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Index doesn't exist yet, that's OK
		}
		return err
	}

	var index CacheIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return err
	}

	c.index.mu.Lock()
	c.index.Products = index.Products
	c.index.TotalSize = index.TotalSize
	c.index.TotalItems = index.TotalItems
	c.index.mu.Unlock()

	return nil
}

// saveIndex saves the cache index to disk
func (c *Cache) saveIndex() error {
	c.index.mu.RLock()
	defer c.index.mu.RUnlock()

	indexPath := filepath.Join(c.config.Dir, "index.json")

	data, err := json.MarshalIndent(c.index, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(indexPath, data, 0644)
}

// SaveProduct saves a product to cache
func (c *Cache) SaveProduct(product *models.Product, source string) error {
	if c == nil || !c.config.Enabled {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	productID := product.ID
	if productID == "" {
		return fmt.Errorf("product ID is required")
	}

	// Check if product already exists
	c.index.mu.RLock()
	existing, exists := c.index.Products[productID]
	c.index.mu.RUnlock()

	// Serialize product to JSON
	productData, err := json.MarshalIndent(product, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal product: %w", err)
	}

	productSize := int64(len(productData))
	productFile := filepath.Join("products", productID+".json")
	productPath := filepath.Join(c.config.Dir, productFile)

	// Check cache size limits before saving
	if err := c.enforceLimits(productSize, productID); err != nil {
		return err
	}

	// Save product file
	if err := os.WriteFile(productPath, productData, 0644); err != nil {
		return fmt.Errorf("failed to write product file: %w", err)
	}

	// Update index
	now := time.Now()
	c.index.mu.Lock()
	if exists {
		// Update existing entry - subtract old size
		c.index.TotalSize -= existing.Size
	} else {
		c.index.TotalItems++
	}

	c.index.Products[productID] = ProductMetadata{
		File:         productFile,
		Size:         productSize,
		CachedAt:     now,
		Source:       source,
		LastAccessed: now,
	}
	c.index.TotalSize += productSize
	c.index.mu.Unlock()

	// Save index
	if err := c.saveIndex(); err != nil {
		log.Warn("Failed to save cache index", "err", err)
	}

	// Call callback if set
	if c.config.OnSave != nil {
		if err := c.config.OnSave(product); err != nil {
			log.Warn("Cache OnSave callback failed", "err", err)
		}
	}

	return nil
}

// enforceLimits enforces cache size and item limits
func (c *Cache) enforceLimits(newItemSize int64, newItemID string) error {
	c.index.mu.Lock()
	defer c.index.mu.Unlock()

	// Check item limit
	if c.config.MaxItems > 0 && c.index.TotalItems >= c.config.MaxItems {
		if c.config.EvictionPolicy == "none" {
			return fmt.Errorf("cache limit reached: %d items (max: %d)", c.index.TotalItems, c.config.MaxItems)
		}
		// Need to evict one item
		if err := c.evictItem(); err != nil {
			return err
		}
	}

	// Check size limit
	if c.config.MaxSize > 0 {
		// Check if adding this item would exceed the limit
		if c.index.TotalSize+newItemSize > c.config.MaxSize {
			if c.config.EvictionPolicy == "none" {
				return fmt.Errorf("cache size limit would be exceeded: %d bytes (max: %d)", c.index.TotalSize+newItemSize, c.config.MaxSize)
			}
			// Evict items until we have enough space
			for c.index.TotalSize+newItemSize > c.config.MaxSize {
				if err := c.evictItem(); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// evictItem evicts one item according to the eviction policy
func (c *Cache) evictItem() error {
	if len(c.index.Products) == 0 {
		return fmt.Errorf("no items to evict")
	}

	var itemToEvict string
	var itemMetadata ProductMetadata

	switch c.config.EvictionPolicy {
	case "lru":
		// Find item with oldest LastAccessed time
		oldestTime := time.Now()
		for id, meta := range c.index.Products {
			if meta.LastAccessed.Before(oldestTime) {
				oldestTime = meta.LastAccessed
				itemToEvict = id
				itemMetadata = meta
			}
		}
	case "fifo":
		// Find item with oldest CachedAt time
		oldestTime := time.Now()
		for id, meta := range c.index.Products {
			if meta.CachedAt.Before(oldestTime) {
				oldestTime = meta.CachedAt
				itemToEvict = id
				itemMetadata = meta
			}
		}
	default:
		// Default to LRU
		oldestTime := time.Now()
		for id, meta := range c.index.Products {
			if meta.LastAccessed.Before(oldestTime) {
				oldestTime = meta.LastAccessed
				itemToEvict = id
				itemMetadata = meta
			}
		}
	}

	if itemToEvict == "" {
		return fmt.Errorf("failed to find item to evict")
	}

	// Delete file
	productPath := filepath.Join(c.config.Dir, itemMetadata.File)
	if err := os.Remove(productPath); err != nil && !os.IsNotExist(err) {
		log.Warn("Failed to remove cached product file", "file", productPath, "err", err)
	}

	// Remove from index
	delete(c.index.Products, itemToEvict)
	c.index.TotalSize -= itemMetadata.Size
	c.index.TotalItems--

	return nil
}

// LoadProduct loads a product from cache
func (c *Cache) LoadProduct(productID string) (*models.Product, error) {
	if c == nil || !c.config.Enabled {
		return nil, fmt.Errorf("cache not enabled")
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	c.index.mu.RLock()
	meta, exists := c.index.Products[productID]
	c.index.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("product not found in cache")
	}

	// Update last accessed time
	now := time.Now()
	c.index.mu.Lock()
	meta.LastAccessed = now
	c.index.Products[productID] = meta
	c.index.mu.Unlock()

	// Load product file
	productPath := filepath.Join(c.config.Dir, meta.File)
	data, err := os.ReadFile(productPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read product file: %w", err)
	}

	var product models.Product
	if err := json.Unmarshal(data, &product); err != nil {
		return nil, fmt.Errorf("failed to unmarshal product: %w", err)
	}

	// Save updated index
	if err := c.saveIndex(); err != nil {
		log.Warn("Failed to save cache index after access", "err", err)
	}

	return &product, nil
}

// LoadAllProducts loads all products from cache
func (c *Cache) LoadAllProducts() ([]*models.Product, error) {
	if c == nil || !c.config.Enabled {
		return nil, fmt.Errorf("cache not enabled")
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	c.index.mu.RLock()
	productIDs := make([]string, 0, len(c.index.Products))
	for id := range c.index.Products {
		productIDs = append(productIDs, id)
	}
	c.index.mu.RUnlock()

	products := make([]*models.Product, 0, len(productIDs))
	for _, id := range productIDs {
		product, err := c.LoadProduct(id)
		if err != nil {
			log.Warn("Failed to load product from cache", "id", id, "err", err)
			continue
		}
		products = append(products, product)
	}

	return products, nil
}

// GetCacheSize returns the current cache size in bytes
func (c *Cache) GetCacheSize() int64 {
	if c == nil || !c.config.Enabled {
		return 0
	}

	c.index.mu.RLock()
	defer c.index.mu.RUnlock()

	return c.index.TotalSize
}

// GetCacheStats returns cache statistics
func (c *Cache) GetCacheStats() (Stats, error) {
	if c == nil || !c.config.Enabled {
		return Stats{}, fmt.Errorf("cache not enabled")
	}

	c.index.mu.RLock()
	defer c.index.mu.RUnlock()

	stats := Stats{
		TotalSize:      c.index.TotalSize,
		TotalItems:     c.index.TotalItems,
		MaxSize:        c.config.MaxSize,
		MaxItems:       c.config.MaxItems,
		EvictionPolicy: c.config.EvictionPolicy,
	}

	if len(c.index.Products) > 0 {
		// Find oldest and newest items
		var oldestTime, newestTime time.Time
		first := true
		for _, meta := range c.index.Products {
			if first {
				oldestTime = meta.CachedAt
				newestTime = meta.CachedAt
				first = false
			} else {
				if meta.CachedAt.Before(oldestTime) {
					oldestTime = meta.CachedAt
				}
				if meta.CachedAt.After(newestTime) {
					newestTime = meta.CachedAt
				}
			}
		}
		stats.OldestItem = oldestTime
		stats.NewestItem = newestTime
	}

	return stats, nil
}

// ClearCache clears all cached products
func (c *Cache) ClearCache() error {
	if c == nil || !c.config.Enabled {
		return fmt.Errorf("cache not enabled")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Delete all product files
	c.index.mu.Lock()
	for _, meta := range c.index.Products {
		productPath := filepath.Join(c.config.Dir, meta.File)
		if err := os.Remove(productPath); err != nil && !os.IsNotExist(err) {
			log.Warn("Failed to remove cached product file", "file", productPath, "err", err)
		}
	}
	c.index.Products = make(map[string]ProductMetadata)
	c.index.TotalSize = 0
	c.index.TotalItems = 0
	c.index.mu.Unlock()

	// Save empty index
	if err := c.saveIndex(); err != nil {
		return err
	}

	return nil
}

// HasProduct checks if a product exists in cache
func (c *Cache) HasProduct(productID string) bool {
	if c == nil || !c.config.Enabled {
		return false
	}

	c.index.mu.RLock()
	defer c.index.mu.RUnlock()

	_, exists := c.index.Products[productID]
	return exists
}

// GetCachedProductIDs returns list of all cached product IDs
func (c *Cache) GetCachedProductIDs() []string {
	if c == nil || !c.config.Enabled {
		return nil
	}

	c.index.mu.RLock()
	defer c.index.mu.RUnlock()

	ids := make([]string, 0, len(c.index.Products))
	for id := range c.index.Products {
		ids = append(ids, id)
	}

	sort.Strings(ids)
	return ids
}
