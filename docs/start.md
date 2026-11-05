# Быстрый старт

Для запуска нужны Python 3.11+, [uv](https://docs.astral.sh/uv/) и Google Chrome. Ozoner запускает Chrome в headless с временным профилем. Вход в аккаунт и перенос cookie не требуются.

## Первая выгрузка { #first-export }

```sh
git clone https://github.com/fresh-milkshake/ozoner.git
cd ozoner
uv sync
uv run ozoner "техника" --limit 40 --output products.csv --log-level INFO
```

Команда запишет до 40 уникальных товаров в `products.csv` в текущем каталоге. Если Chrome установлен по нестандартному пути, передайте [`--chrome`](cli.md#options). Если строк меньше, чем ожидалось, проверьте [ограничение страниц](troubleshooting.md#fewer-results).

## Первый запрос из Python { #first-python }

```python
from ozoner import OzonClient, write_csv

client = OzonClient()
products = client.search("беспроводные наушники", limit=20)
if products:
    print(products[0].name, products[0].price)
    write_csv(products, "headphones.csv")
```

Каждый вызов открывает временную сессию. Для нескольких карточек используйте [`get_products()`](reference.md#ozoner.OzonClient.get_products), чтобы загрузить их в одной сессии.

Дальше можно [выгрузить категорию](api.md#categories), [применить фильтры витрины](filters.md) или [настроить логирование](cli.md#logging).
