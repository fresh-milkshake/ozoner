# CLI и логирование

```sh
uv run ozoner "техника" --limit 40 --max-pages 10 --output products.csv --log-level INFO
uv run ozoner "наушники" --filter "price=1000;10000" --sort price
```

Параметр `--chrome` задаёт путь к Chrome. `--timeout` ограничивает отдельную операцию браузера в секундах. CLI пишет логи в stderr, итоговую строку в stdout и возвращает ненулевой код при ошибке. При ошибке получения данных CSV не заменяется.

Повторите `--filter KEY=VALUE`, чтобы передать несколько фильтров. Значения для одного ключа можно перечислить через запятую.

Приложение может настроить логгер `ozoner` через стандартный Python `logging`.

```python
import logging

logging.basicConfig(level=logging.WARNING)
logging.getLogger("ozoner").setLevel(logging.DEBUG)
```

На уровне `INFO` видны начало операции, статус, число результатов и время. `DEBUG` добавляет номера страниц и ID успешно прочитанных карточек. `WARNING` сообщает об отказах и ошибках чтения. События Ozoner не включают cookie, тело ответа и параметры пагинации.

Собрать документацию локально можно командой `uvx --from mkdocs==1.6.1 mkdocs build --strict`.
