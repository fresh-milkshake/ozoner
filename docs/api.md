# Python API

```python
from ozoner import OzonClient, OzonHTTPError, write_csv

client = OzonClient(timeout=25)
products = client.search("наушники", limit=20, max_price=10_000)
write_csv(products, "headphones.csv")

category = client.get_category(
    "https://www.ozon.ru/category/smartfony-15502/apple-26303000/", limit=20
)
write_csv(category.products, "phones.csv")

facets = client.get_filters("наушники")
for facet in facets:
    print(facet.key, facet.label, facet.options)

filtered = client.search("наушники", filters={"price": "1000;10000"}, sort="price")

try:
    details = client.get_products(products[:3])
except OzonHTTPError as error:
    print(error.status)
else:
    for item in details:
        print(item.name, item.price, item.seller)
```

| Метод | Поведение |
| --- | --- |
| `search(query, limit=40, max_pages=10, min_price=None, max_price=None)` | Возвращает уникальные результаты в порядке витрины. Фильтр применяется к прочитанным страницам, поэтому лимит страниц может ограничить число совпадений. |
| `get_product(reference)` | Принимает числовой ID, HTTPS-ссылку на карточку Ozon или объект `Product`. |
| `get_products(references)` | Обрабатывает уникальные ID в порядке ввода, переиспользуя один браузер. При сбое прерывает весь пакет. |
| `get_category(url, limit=40, max_pages=10)` | Возвращает `Category` с ID, названием, URL и уникальными товарами категории. Сохраняет фильтры входной ссылки при запросе. |
| `get_filters(query=None, category_url=None, filters=None)` | Получает ключи фильтров, доступные значения, выбранные варианты и границы диапазонов для запроса или категории. |
| `write_csv(products, path)` | Записывает CSV с полями выдачи в UTF-8 с BOM и разделителем `;`. Заменяет файл после успешной записи. |

В `Product` доступны ID, название, URL, цены, рейтинг, число отзывов и ссылки на изображения. `ProductDetails` дополняет их брендом, описанием, продавцом и его URL, доступностью, валютой, ценой с Ozon Картой, краткими характеристиками, цепочкой категорий и видео. Изображения объединяются из структурированных данных и галереи. Поля зависят от содержимого карточки. Цены представлены типом `Decimal`.

Неверные параметры приводят к `ValueError`, ошибки браузера и разбора — к `OzonError`. `OzonHTTPError` наследуется от `OzonError` и хранит HTTP-статус.

Параметр `filters` у `search()` и `get_category()` принимает ключи витрины. Передайте один ID, список ID или диапазон в виде строки `"1000;10000"`. Для категории новые значения заменяют одноимённые параметры исходной ссылки. Параметр `sort` передаётся витрине как `sorting`. Аргументы `min_price` и `max_price` у `search()` дополнительно фильтруют цены в прочитанных карточках на стороне клиента.

Список фильтров зависит от категории, запроса, региона и выбранных значений. Ozon может проигнорировать параметры, которых нет у текущей выдачи, либо применить диапазон к другой цене, чем показана в карточке.
