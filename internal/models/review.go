package models

// Review represents a product review
type Review struct {
	ID          string   `json:"id"`
	ProductID   string   `json:"product_id"`
	Author      string   `json:"author"`
	Rating      int      `json:"rating"` // 1-5
	Title       string   `json:"title"`
	Text        string   `json:"text"`
	Date        string   `json:"date"`
	Verified    bool     `json:"verified"`
	Helpful     int      `json:"helpful"`
	Images      []string `json:"images,omitempty"`
	Pros        []string `json:"pros,omitempty"`
	Cons        []string `json:"cons,omitempty"`
}

// ReviewResponse represents paginated review response
type ReviewResponse struct {
	Reviews []Review `json:"reviews"`
	Total   int      `json:"total"`
	Page    int      `json:"page"`
	Pages   int      `json:"pages"`
}

