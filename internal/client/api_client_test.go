package client

import (
	"testing"
)

func TestExtractProductIDFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "Standard product URL",
			url:      "https://www.ozon.ru/product/iphone-15-pro-256gb-128gb-512gb-1tb-128gb-natural-titanium-123456789/",
			expected: "123456789",
		},
		{
			name:     "Product URL without trailing slash",
			url:      "https://www.ozon.ru/product/iphone-15-pro-123456789",
			expected: "123456789",
		},
		{
			name:     "Invalid URL",
			url:      "https://www.ozon.ru/",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractProductIDFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("ExtractProductIDFromURL(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestExtractSellerIDFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "Standard seller URL",
			url:      "https://www.ozon.ru/seller/some-seller-name-12345/",
			expected: "some-seller-name-12345",
		},
		{
			name:     "Seller URL without trailing slash",
			url:      "https://www.ozon.ru/seller/seller-id",
			expected: "seller-id",
		},
		{
			name:     "Invalid URL",
			url:      "https://www.ozon.ru/",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractSellerIDFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("ExtractSellerIDFromURL(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}

