package models

// Product represents a product from Ozon
type Product struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       float64  `json:"price"`
	OldPrice    *float64 `json:"old_price,omitempty"`
	Currency    string   `json:"currency"`
	Rating      float64  `json:"rating"`
	ReviewsCount int     `json:"reviews_count"`
	Images      []string `json:"images"`
	URL         string   `json:"url"`
	SKU         string   `json:"sku"`
	Category    string   `json:"category"`
	Brand       string   `json:"brand"`
	SellerID    string   `json:"seller_id"`
	SellerName  string   `json:"seller_name"`
	InStock     bool     `json:"in_stock"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
}

// ProductSearchResult represents a product in search results
type ProductSearchResult struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Price       float64  `json:"price"`
	OldPrice    *float64 `json:"old_price,omitempty"`
	Currency    string   `json:"currency"`
	Rating      float64  `json:"rating"`
	ReviewsCount int     `json:"reviews_count"`
	Image       string   `json:"image"`
	URL         string   `json:"url"`
	SKU         string   `json:"sku"`
	InStock     bool     `json:"in_stock"`
}

// SearchFilters represents filters for product search
type SearchFilters struct {
	MinPrice    *float64 `json:"min_price,omitempty"`
	MaxPrice    *float64 `json:"max_price,omitempty"`
	Category    string   `json:"category,omitempty"`
	Brand       string   `json:"brand,omitempty"`
	Rating      *float64 `json:"rating,omitempty"`
	InStock     *bool    `json:"in_stock,omitempty"`
	SortBy      string   `json:"sort_by,omitempty"` // price_asc, price_desc, rating, popularity
	Page        int      `json:"page,omitempty"`
	PageSize    int      `json:"page_size,omitempty"`
}

