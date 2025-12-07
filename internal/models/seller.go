package models

// Seller represents a seller/shop on Ozon
type Seller struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Rating      float64 `json:"rating"`
	ReviewsCount int    `json:"reviews_count"`
	ProductsCount int   `json:"products_count"`
	URL         string  `json:"url"`
	Logo        string  `json:"logo"`
	IsVerified  bool    `json:"is_verified"`
	JoinedDate  string  `json:"joined_date,omitempty"`
}

