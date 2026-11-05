# Ozoner

Неофициальный Python-клиент публичной витрины Ozon. Ищет товары, читает карточки и сохраняет выдачу в CSV. Для запуска нужны Python 3.11+, uv и установленный Google Chrome. Scrapling использует временный профиль в headless.

```sh
git clone https://github.com/fresh-milkshake/ozoner.git
cd ozoner
uv sync
uv run ozoner "техника" --limit 40 --output products.csv --log-level INFO
```

Ассортимент, продавец и цены зависят от региона и сессии. Витрина может ответить HTTP 403 даже после успешного поиска. Точный статус доступен в `OzonHTTPError.status`.

См. [Python API](api.md) и [CLI и логирование](cli.md).
