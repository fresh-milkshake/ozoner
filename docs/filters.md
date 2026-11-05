# Фильтры и сортировка

Ключи и значения фильтров задаёт Ozon. Они зависят от запроса, категории, региона и уже выбранных вариантов. Используйте [`get_filters()`](reference.md#ozoner.OzonClient.get_filters), чтобы получить ключи и ID, которые витрина предлагает для конкретной выдачи.

## Узнать доступные фильтры { #discover }

```python
from ozoner import OzonClient

client = OzonClient()
for facet in client.get_filters("наушники"):
    print(facet.key, facet.label, facet.minimum, facet.maximum)
    for option in facet.options:
        print(option.value, option.label, option.selected)
```

Для категории передайте [`category_url=`](reference.md#ozoner.OzonClient.get_filters) вместо поисковой строки. Чтобы посмотреть варианты с учётом уже применённых условий, передайте [`filters=`](reference.md#ozoner.OzonClient.get_filters). Метод возвращает [`StorefrontFilter`](reference.md#ozoner.StorefrontFilter) с вариантами [`FilterOption`](reference.md#ozoner.FilterOption). Если Ozon отклонил запрос, смотрите [разбор HTTP 403](troubleshooting.md#http-403).

## Применить условия { #apply }

```python
products = client.search(
    "наушники",
    filters={"price": "1000;10000"},
    sort="price",
    limit=20,
)
```

| Значение в [`filters`](reference.md#ozoner.OzonClient.search) | Что передаётся витрине |
| --- | --- |
| Один ID | Одно выбранное значение |
| Список ID | Значения через запятую |
| Строка `"1000;10000"` | Диапазон |

[`sort`](reference.md#ozoner.OzonClient.search) передаётся как параметр `sorting`. При [выгрузке категории](api.md#categories) одноимённые параметры из [`filters`](reference.md#ozoner.OzonClient.get_category) заменяют значения из исходной ссылки.

## Проверить цену результата { #local-price }

Ozon может применить диапазон к другой цене, чем показана в карточке. Если нужна строгая граница по [`Product.price`](reference.md#ozoner.Product.price), задайте `min_price` и `max_price` в [`search()`](reference.md#ozoner.OzonClient.search). Товары без цены не проходят такую проверку. Локальный фильтр проверяет только прочитанные страницы.

См. также [фильтры в CLI](cli.md#options) и [лимиты выдачи](troubleshooting.md#fewer-results).
