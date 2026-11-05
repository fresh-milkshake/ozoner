# CLI и логирование

Установка и первый запуск описаны в [быстром старте](start.md).

```sh
uv run ozoner "техника" --limit 40 --max-pages 10 --output products.csv --log-level INFO
uv run ozoner "наушники" --filter "price=1000;10000" --sort price
```

## Параметры { #options }

| Параметр | По умолчанию | Назначение |
| --- | --- | --- |
| `--limit` | `40` | Максимум уникальных товаров |
| `--max-pages` | `10` | Максимум страниц выдачи |
| `-o`, `--output` | `products.csv` | Путь к CSV |
| `--timeout` | `25` | Таймаут отдельной операции в секундах |
| `--chrome` | Автовыбор | Путь к Chrome |
| `--filter KEY=VALUE` | — | Повторяемый параметр [фильтров витрины](filters.md#apply) |
| `--sort` | Витрина | [Ключ сортировки](filters.md#apply) |
| `--log-level` | `WARNING` | [Уровень логирования](#logging) |

## CSV { #csv }

`write_csv()` в Python и CLI используют UTF-8 с BOM и разделитель `;`. Файл содержит поля поисковой выдачи. При ошибке получения данных существующий CSV сохраняется. См. [ошибки экспорта](troubleshooting.md#csv).

## Логирование { #logging }

CLI пишет логи в stderr, итоговую строку в stdout и возвращает ненулевой код при ошибке.

Приложение может настроить логгер `ozoner` через стандартный Python `logging`.

```python
import logging

logging.basicConfig(level=logging.WARNING)
logging.getLogger("ozoner").setLevel(logging.DEBUG)
```

На уровне `INFO` видны начало операции, статус, число результатов и время. `DEBUG` добавляет номера страниц и ID успешно прочитанных карточек. `WARNING` сообщает об отказах и ошибках чтения. События Ozoner не включают cookie, тело ответа и параметры пагинации.

Собрать документацию локально можно командой `uv run --locked --only-group docs mkdocs build --strict`.
