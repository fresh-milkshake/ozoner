# Ozoner - Enterprise-level Ozon.ru Parser Library

Полнофункциональная библиотека на Go для парсинга данных с сайта Ozon.ru. Предоставляет удобный API для поиска товаров, получения информации о товарах, отзывов и данных о магазинах.

## Особенности

- 🔍 **Поиск товаров** с поддержкой фильтров (цена, категория, бренд, рейтинг и т.д.)
- 📦 **Детальная информация о товарах** включая характеристики, изображения, цены
- ⭐ **Отзывы о товарах** с пагинацией
- 🏪 **Информация о магазинах** и их товарах
- 🚀 **Гибридный подход**: API endpoints + headless браузер для обхода защиты
- ⚡ **Rate limiting** и автоматические retry для надежности
- 🔄 **Конкурентная обработка** через worker pool
- 🛡️ **Enterprise-level** архитектура с обработкой ошибок и логированием

## Установка

```bash
go get ozoner
```

## Быстрый старт

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "ozoner/pkg/ozoner"
)

func main() {
    // Создание клиента с настройками по умолчанию
    client, err := ozoner.New(ozoner.DefaultConfig())
    if err != nil {
        panic(err)
    }
    defer client.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Поиск товаров
    products, err := client.SearchProducts(ctx, "iphone", nil)
    if err != nil {
        panic(err)
    }

    for _, product := range products {
        fmt.Printf("%s: %.2f %s\n", product.Name, product.Price, product.Currency)
    }
}
```

## API Документация

### Конфигурация

```go
type Config struct {
    RateLimit     float64       // Лимит запросов в секунду (по умолчанию 10.0)
    RetryAttempts int           // Количество попыток при ошибке (по умолчанию 3)
    Timeout       time.Duration // Таймаут запросов (по умолчанию 30s)
    UseBrowser    bool          // Использовать headless браузер (по умолчанию false)
    Headless      bool          // Headless режим браузера (по умолчанию true)
    UserAgent     string        // User-Agent для запросов
    Headers       map[string]string // Дополнительные заголовки
    WorkerCount   int           // Количество воркеров для конкурентной обработки
}
```

### Основные методы

#### SearchProducts

Поиск товаров по ключевому слову с опциональными фильтрами.

```go
func (o *Ozoner) SearchProducts(ctx context.Context, query string, filters *SearchFilters) ([]Product, error)
```

**Параметры:**
- `query` - поисковый запрос
- `filters` - фильтры поиска (может быть nil)

**Пример:**

```go
filters := &ozoner.SearchFilters{
    MinPrice: floatPtr(10000),
    MaxPrice: floatPtr(100000),
    InStock:  boolPtr(true),
    SortBy:   "price_asc",
    Page:     1,
    PageSize: 20,
}

products, err := client.SearchProducts(ctx, "ноутбук", filters)
```

#### GetProduct

Получение детальной информации о товаре по ID.

```go
func (o *Ozoner) GetProduct(ctx context.Context, productID string) (*Product, error)
```

**Пример:**

```go
product, err := client.GetProduct(ctx, "123456789")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Product: %s\n", product.Name)
fmt.Printf("Price: %.2f %s\n", product.Price, product.Currency)
fmt.Printf("Rating: %.1f (%d reviews)\n", product.Rating, product.ReviewsCount)
```

#### GetProductReviews

Получение отзывов о товаре.

```go
func (o *Ozoner) GetProductReviews(ctx context.Context, productID string, page int) (*ReviewResponse, error)
```

**Пример:**

```go
reviews, err := client.GetProductReviews(ctx, "123456789", 1)
if err != nil {
    log.Fatal(err)
}

for _, review := range reviews.Reviews {
    fmt.Printf("Author: %s, Rating: %d/5\n", review.Author, review.Rating)
}
```

#### GetSeller

Получение информации о магазине.

```go
func (o *Ozoner) GetSeller(ctx context.Context, sellerID string) (*Seller, error)
```

**Пример:**

```go
seller, err := client.GetSeller(ctx, "seller-id")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Seller: %s\n", seller.Name)
fmt.Printf("Rating: %.1f\n", seller.Rating)
fmt.Printf("Products: %d\n", seller.ProductsCount)
```

#### GetSellerProducts

Получение всех товаров магазина.

```go
func (o *Ozoner) GetSellerProducts(ctx context.Context, sellerID string, page int) ([]Product, error)
```

**Пример:**

```go
products, err := client.GetSellerProducts(ctx, "seller-id", 1)
if err != nil {
    log.Fatal(err)
}

for _, product := range products {
    fmt.Printf("%s: %.2f %s\n", product.Name, product.Price, product.Currency)
}
```

### Вспомогательные функции

#### ExtractProductIDFromURL

Извлечение ID товара из URL.

```go
url := "https://www.ozon.ru/product/iphone-15-pro-256gb-128gb-512gb-1tb-128gb-natural-titanium-123456789/"
productID := ozoner.ExtractProductIDFromURL(url)
```

#### ExtractSellerIDFromURL

Извлечение ID магазина из URL.

```go
url := "https://www.ozon.ru/seller/some-seller-name-12345/"
sellerID := ozoner.ExtractSellerIDFromURL(url)
```

## Модели данных

### Product

```go
type Product struct {
    ID           string                 `json:"id"`
    Name         string                 `json:"name"`
    Description  string                 `json:"description"`
    Price        float64                `json:"price"`
    OldPrice     *float64               `json:"old_price,omitempty"`
    Currency     string                 `json:"currency"`
    Rating       float64                `json:"rating"`
    ReviewsCount int                    `json:"reviews_count"`
    Images       []string               `json:"images"`
    URL          string                 `json:"url"`
    SKU          string                 `json:"sku"`
    Category     string                 `json:"category"`
    Brand        string                 `json:"brand"`
    SellerID     string                 `json:"seller_id"`
    SellerName   string                 `json:"seller_name"`
    InStock      bool                   `json:"in_stock"`
    Attributes   map[string]interface{}  `json:"attributes,omitempty"`
}
```

### SearchFilters

```go
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
```

### Review

```go
type Review struct {
    ID        string   `json:"id"`
    ProductID string   `json:"product_id"`
    Author    string   `json:"author"`
    Rating    int      `json:"rating"` // 1-5
    Title     string   `json:"title"`
    Text      string   `json:"text"`
    Date      string   `json:"date"`
    Verified  bool     `json:"verified"`
    Helpful   int      `json:"helpful"`
    Images    []string `json:"images,omitempty"`
    Pros      []string `json:"pros,omitempty"`
    Cons      []string `json:"cons,omitempty"`
}
```

### Seller

```go
type Seller struct {
    ID           string  `json:"id"`
    Name         string  `json:"name"`
    Description  string  `json:"description"`
    Rating       float64 `json:"rating"`
    ReviewsCount int     `json:"reviews_count"`
    ProductsCount int    `json:"products_count"`
    URL          string  `json:"url"`
    Logo         string  `json:"logo"`
    IsVerified   bool    `json:"is_verified"`
    JoinedDate   string  `json:"joined_date,omitempty"`
}
```

## Примеры использования

Библиотека включает несколько примеров в директории `examples/`:

- `examples/search/main.go` - примеры поиска товаров
- `examples/product/main.go` - работа с товарами и отзывами
- `examples/seller/main.go` - работа с магазинами
- `examples/advanced/main.go` - продвинутые примеры (batch обработка, пагинация)

Запуск примеров:

```bash
go run examples/search/main.go
go run examples/product/main.go
go run examples/seller/main.go
go run examples/advanced/main.go
```

## Архитектура

Библиотека использует гибридный подход:

1. **API Client** - обращается к reverse-engineered API endpoints Ozon для быстрых запросов
2. **Browser Client** - использует headless браузер (chromedp) для обхода антибот-защиты и динамического контента
3. **HTTP Client** - управляет rate limiting, retry механизмом и конфигурацией запросов
4. **Parsers** - извлекают структурированные данные из API ответов
5. **Worker Pool** - обеспечивает конкурентную обработку множественных запросов

## Обработка ошибок

Библиотека использует стандартные Go ошибки. Все методы возвращают `error` в качестве последнего параметра. Рекомендуется всегда проверять ошибки:

```go
product, err := client.GetProduct(ctx, productID)
if err != nil {
    log.Printf("Error: %v", err)
    return
}
```

## Rate Limiting

Библиотека автоматически управляет rate limiting через token bucket алгоритм. По умолчанию установлен лимит 10 запросов в секунду. Можно настроить через `Config.RateLimit`.

## Retry механизм

Автоматический retry с exponential backoff для:
- Сетевых ошибок
- HTTP статусов: 429 (Too Many Requests), 500, 502, 503, 504

Количество попыток настраивается через `Config.RetryAttempts` (по умолчанию 3).

## Контекст

Все методы принимают `context.Context` для:
- Отмены операций
- Установки таймаутов
- Передачи метаданных

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

products, err := client.SearchProducts(ctx, "iphone", nil)
```

## Зависимости

- `github.com/chromedp/chromedp` - headless браузер
- `golang.org/x/time/rate` - rate limiting
- `github.com/PuerkitoBio/goquery` - HTML парсинг (опционально)

## Лицензия

MIT

## Вклад в проект

Приветствуются pull requests и issues. Пожалуйста, убедитесь, что код соответствует стандартам Go и проходит все тесты.

## Важные замечания

### Антибот-защита Ozon

Ozon.ru использует продвинутую антибот-защиту, которая может блокировать прямые API запросы (HTTP 403). В таких случаях рекомендуется использовать browser client:

```go
config := ozoner.DefaultConfig()
config.UseBrowser = true  // Включить headless браузер
client, err := ozoner.New(config)
```

**Требования для browser client:**
- Установленный Chrome/Chromium
- Больше ресурсов (память, CPU)
- Медленнее, чем прямые API запросы

### Рекомендации

- **Для разработки/тестирования**: Используйте browser client (`UseBrowser: true`)
- **Для production**: Начните с API, переключитесь на browser при получении 403
- **Rate limiting**: Используйте разумные лимиты (5-10 запросов/сек) для избежания блокировок
- **Прокси**: Рассмотрите использование прокси для высоконагруженных сценариев
- **Соблюдайте ToS**: Не злоупотребляйте запросами, соблюдайте условия использования Ozon.ru

### Обработка ошибок

Библиотека автоматически определяет блокировку (403) и предлагает использовать browser client:

```go
products, err := client.SearchProducts(ctx, "iphone", nil)
if err != nil {
    if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "forbidden") {
        // Включите browser client в конфигурации
        log.Println("API blocked, use browser client")
    }
}
```

### Другие замечания

- Библиотека использует reverse-engineered API endpoints, которые могут измениться
- При использовании headless браузера требуется Chrome/Chromium
- Cookie jar автоматически управляется для поддержания сессий

## Поддержка

Для вопросов и проблем создавайте issues в репозитории проекта.

