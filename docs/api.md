# Python API

Начните с [установки](start.md). Все методы используют публичную витрину Ozon. Для Chrome вне стандартного каталога передайте `OzonClient(chrome_path=...)`.

Точные сигнатуры, типы и исходники публичных объектов собраны в [справочнике по коду](reference.md).

## Поиск товаров { #search }

```python
from ozoner import OzonClient, write_csv

client = OzonClient(timeout=25)
products = client.search("наушники", limit=20, max_price=10_000)
write_csv(products, "headphones.csv")
```

`search()` возвращает уникальные товары в порядке витрины. См. [сигнатуру метода](reference.md#ozoner.OzonClient.search), [фильтры и сортировку](filters.md) и [лимиты выдачи](troubleshooting.md#fewer-results).

## Категории { #categories }

```python
category = client.get_category(
    "https://www.ozon.ru/category/smartfony-15502/apple-26303000/", limit=20
)
print(category.id, category.name)
write_csv(category.products, "phones.csv")
```

[`Category`](reference.md#ozoner.Category) содержит ID, название, URL и товары. `get_category()` принимает параметры исходной ссылки и умеет [заменять фильтры](filters.md#apply) через `filters=`.

## Карточки товаров { #product-details }

[`get_product()`](reference.md#ozoner.OzonClient.get_product) принимает числовой ID, ссылку Ozon или объект `Product`. [`get_products()`](reference.md#ozoner.OzonClient.get_products) обрабатывает уникальные ID в порядке ввода и переиспользует один браузер. Если одна карточка не загрузилась, пакет завершается ошибкой.

```python
from ozoner import OzonHTTPError

try:
    details = client.get_products(products[:3])
except OzonHTTPError as error:
    print(error.status)
else:
    for item in details:
        print(item.name, item.price, item.seller)
```

## Модели данных { #models }

В `Product` доступны ID, название, URL, цены, рейтинг, число отзывов и ссылки на изображения. `ProductDetails` дополняет их брендом, описанием, продавцом и его URL, доступностью, валютой, ценой с Ozon Картой, краткими характеристиками, цепочкой категорий и видео. Изображения объединяются из структурированных данных и галереи. Поля зависят от содержимого карточки. Цены представлены типом `Decimal`.

## Ошибки { #errors }

Неверные параметры приводят к `ValueError`. Ошибки браузера и разбора приводят к `OzonError`. `OzonHTTPError` наследуется от `OzonError` и хранит HTTP-статус в `status`.

См. [частые ошибки](troubleshooting.md), [настройку логов](cli.md#logging) и [формат CSV](cli.md#csv).
