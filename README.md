# Ozoner

Неофициальная Python-библиотека для работы с публичной витриной [Ozon](https://www.ozon.ru/). Поиск товаров, цены, карточки и экспорт выдачи в CSV.

![Python](https://img.shields.io/badge/Python-3.11%2B-3776AB?style=for-the-badge&logo=python&logoColor=white)
![Scrapling](https://img.shields.io/badge/Scrapling-0.5.1-6842C2?style=for-the-badge)
![uv](https://img.shields.io/badge/uv-261230?style=for-the-badge&logo=uv&logoColor=white)

## Возможности

- Поиск с пагинацией, удалением дублей, сортировкой и фильтрами витрины. Доступные фильтры поиска и категории можно получить через `get_filters()`.
- Карточка по ID, ссылке или результату поиска. Пакетный запрос повторно использует один браузер.
- Выгрузка категории по ссылке с её названием, ID и списком товаров.
- Цены, рейтинг, продавец, краткие характеристики, категории, галерея и видео из карточки, когда витрина их отдаёт.
- CSV и логи через стандартный модуль Python `logging`.

## Запуск

Python 3.11+, uv и установленный Google Chrome. Scrapling запускает Chrome в headless с временным профилем.

```sh
uv sync
uv run ozoner "техника" --limit 40 --output products.csv --log-level INFO
```

Если Chrome установлен в нестандартном месте, передайте `--chrome /path/to/chrome`.

```python
from ozoner import OzonClient, write_csv

client = OzonClient()
products = client.search("беспроводные наушники", limit=40, max_price=10_000)
write_csv(products, "headphones.csv")

if products:
    details = client.get_product(products[0])
    print(details.name, details.price, details.seller)
```

`search()` возвращает товары в порядке выдачи, убирая дубли по ID. Последняя страница или лимит страниц могут дать меньше строк, чем запрошено. Сетевой отказ или нечитаемая выдача приводят к `OzonError`. Цена хранится как `Decimal` в рублях. Недоступные цена, рейтинг и число отзывов представлены как `None`.

CSV содержит поля поисковой выдачи. Цены и ассортимент зависят от региона и сессии витрины. Ozon может ограничить запросы к выдаче или карточкам. В этом случае библиотека поднимает `OzonHTTPError` со статусом ответа.

## Проверки

```sh
uv run ruff check .
uv run ruff format --check .
uv build
```

## Документация

[Документация Ozoner](https://fresh-milkshake.github.io/ozoner/) включает примеры Python API, команды CLI и настройку логирования. Исходники сайта находятся в ветке [mkdocs](https://github.com/fresh-milkshake/ozoner/tree/mkdocs).

## Автор и лицензия

[fresh-milkshake](https://github.com/fresh-milkshake). Лицензия [MIT](LICENSE).
