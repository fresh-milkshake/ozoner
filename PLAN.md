# План реализации чекпоинтов и кеширования

## Задачи

### 1. Создание системы кеширования результатов (internal/cache)

**Файл**: `internal/cache/cache.go`

Создать пакет для автоматического кеширования всех полученных результатов:

- Структура `Cache` с методами сохранения/загрузки
- Автоматическое сохранение каждого продукта сразу после получения
- Хранение в JSON файлах в директории `.ozoner/cache/`
- Методы: `SaveProduct(product)`, `LoadProduct(id)`, `LoadAllProducts()`, `ClearCache()`, `GetCacheSize()`, `GetCacheStats()`
- Поддержка метаданных (timestamp, source query, etc.)
- **Ограничение размера кеша**: настраиваемый максимальный размер (по умолчанию 100MB)
- **LRU eviction**: при превышении лимита удаляются самые старые записи
- **Возможность отключения**: через конфигурацию `CacheConfig.Enabled = false`

**Структура кеша**:

- Каждый продукт сохраняется в файл `.ozoner/cache/products/{productID}.json`
- Индексный файл `.ozoner/cache/index.json` со списком всех закешированных продуктов и их метаданными
- Файл статистики `.ozoner/cache/stats.json` с информацией о размере кеша

### 2. Создание системы чекпоинтов (internal/checkpoint)

**Файл**: `internal/checkpoint/checkpoint.go`

Создать пакет для управления чекпоинтами:

- Структура `Checkpoint` для сохранения состояния процесса
- Структура `CheckpointConfig` для настройки (частота сохранения, путь, формат)
- Методы: `Save(checkpoint)`, `Load(id)`, `List()`, `Delete(id)`, `GetLatest()`
- Хранение в JSON файлах в директории `.ozoner/checkpoints/`
- Чекпоинт содержит: список обработанных ID, текущий прогресс, параметры запроса, timestamp
- **Автосохранение после каждого продукта** (настраиваемо через конфигурацию)

**Checkpoint структура**:

```go
type Checkpoint struct {
    ID          string
    CreatedAt   time.Time
    UpdatedAt   time.Time
    Query       string
    Filters     *SearchFilters
    ProcessedIDs []string
    CurrentPage  int
    TotalTarget  int
    State       map[string]interface{} // для пользовательских данных
}
```

### 3. Расширение конфигурации (pkg/ozoner/ozoner.go)

**Изменения в `Config`**:

Добавить `CheckpointConfig`:

- `Enabled bool` - включить/выключить чекпоинты (по умолчанию true)
- `Dir string` - директория для чекпоинтов (по умолчанию `.ozoner/checkpoints`)
- `AutoSave bool` - автоматическое сохранение после каждого продукта (по умолчанию true)
- `OnSave func(checkpoint *Checkpoint) error` - callback при сохранении чекпоинта

Добавить `CacheConfig`:

- `Enabled bool` - включить/выключить кеш (по умолчанию true)
- `Dir string` - директория для кеша (по умолчанию `.ozoner/cache`)
- `MaxSize int64` - максимальный размер кеша в байтах (по умолчанию 100MB = 100*1024*1024)
- `MaxItems int` - максимальное количество элементов в кеше (0 = без ограничений, по умолчанию 0)
- `EvictionPolicy string` - политика удаления при переполнении: "lru" (по умолчанию), "fifo", "none" (ошибка при переполнении)
- `AutoSave bool` - автоматическое сохранение каждого продукта (по умолчанию true)
- `OnSave func(product *Product) error` - callback при сохранении продукта

### 4. Интеграция в основной клиент (pkg/ozoner/ozoner.go)

**Изменения**:

- Добавить поля `cache *cache.Cache` и `checkpointManager *checkpoint.Manager` в структуру `Ozoner`
- В методе `New()` инициализировать cache и checkpoint если они включены
- В методах `SearchProducts()` и `GetProduct()`:
  - Сначала проверять кеш (если включен)
  - Автоматически сохранять результаты в кеш после получения
  - Сохранять чекпоинт после каждого продукта (если включено автосохранение)
- Добавить публичные методы:
  - `SaveCheckpoint(name string, state map[string]interface{}) error` - ручное сохранение чекпоинта
  - `LoadCheckpoint(name string) (*checkpoint.Checkpoint, error)` - загрузка чекпоинта
  - `ListCheckpoints() ([]checkpoint.Checkpoint, error)` - список всех чекпоинтов
  - `GetCacheStats() (cache.Stats, error)` - статистика кеша
  - `ClearCache() error` - очистка кеша

### 5. Модификация тестового скрипта (test/test_technika_csv.go)

**Изменения**:

- Открывать CSV файл в режиме append с самого начала (`os.OpenFile` с флагом `os.O_APPEND|os.O_CREATE|os.O_WRONLY`)
- Проверять существование файла и записывать заголовки только если файл новый
- Записывать каждый продукт в CSV сразу после получения детальной информации (не ждать все продукты)
- Интегрировать чекпоинты:
  - При запуске проверять наличие чекпоинтов и предлагать восстановление
  - Восстанавливать состояние из чекпоинта (пропускать уже обработанные товары)
  - Сохранять чекпоинт после каждого продукта автоматически
  - Сохранять прогресс: какие товары уже обработаны, текущая страница поиска
- Использовать кеш для восстановления уже полученных продуктов (избегать повторных запросов)
- Обработка сигналов (SIGINT, SIGTERM) для корректного сохранения чекпоинта при прерывании

### 6. Детали реализации кеша

**Управление размером**:

- При сохранении продукта проверять текущий размер кеша
- Если размер превышает `MaxSize`, применять политику eviction:
  - LRU: удалять продукты с самым старым временем последнего доступа
  - FIFO: удалять продукты в порядке добавления
  - None: возвращать ошибку при переполнении
- Если установлен `MaxItems`, ограничивать количество элементов
- Метод `GetCacheSize()` возвращает текущий размер в байтах
- Метод `GetCacheStats()` возвращает статистику: размер, количество элементов, самый старый/новый элемент

**Структура файлов кеша**:

- `.ozoner/cache/products/{productID}.json` - файлы продуктов
- `.ozoner/cache/index.json` - индекс с метаданными:
  ```json
  {
    "products": {
      "productID": {
        "file": "products/productID.json",
        "size": 1234,
        "cached_at": "2025-01-01T12:00:00Z",
        "source": "GetProduct",
        "last_accessed": "2025-01-01T12:00:00Z"
      }
    },
    "total_size": 123456,
    "total_items": 10
  }
  ```


### 7. Детали реализации чекпоинтов

**Структура файлов**:

- `.ozoner/checkpoints/{checkpointID}.json` - файлы чекпоинтов
- Checkpoint ID генерируется автоматически (UUID или timestamp-based) или задается пользователем
- Поддержка нескольких чекпоинтов одновременно
- Метод `GetLatest()` возвращает последний созданный чекпоинт

**Восстановление из чекпоинта**:

- При загрузке чекпоинта восстанавливается список обработанных ID
- Пропускаются товары, которые уже есть в списке `ProcessedIDs`
- Восстанавливается текущая страница поиска и параметры запроса
- Можно продолжить с того места, где остановились

## Файлы для создания/изменения

1. `internal/cache/cache.go` - новый файл, система кеширования с ограничениями размера
2. `internal/checkpoint/checkpoint.go` - новый файл, система чекпоинтов
3. `pkg/ozoner/ozoner.go` - добавить поддержку cache и checkpoint в Config и Ozoner
4. `test/test_technika_csv.go` - модифицировать для записи по мере получения и интеграции чекпоинтов

## Примеры использования

**Настройка кеша с ограничениями**:

```go
config := ozoner.DefaultConfig()
config.CacheConfig.Enabled = true
config.CacheConfig.MaxSize = 50 * 1024 * 1024 // 50MB
config.CacheConfig.MaxItems = 1000
config.CacheConfig.EvictionPolicy = "lru"
```

**Отключение кеша**:

```go
config.CacheConfig.Enabled = false
```

**Работа с чекпоинтами в тестовом скрипте**:

```go
// При запуске проверка чекпоинтов
checkpoints, _ := client.ListCheckpoints()
if len(checkpoints) > 0 {
    // Предложить восстановление
    latest := checkpoints[len(checkpoints)-1]
    // Восстановить состояние
}
```