"""Write catalog summaries as a spreadsheet-friendly CSV."""

import csv
from collections.abc import Iterable
from pathlib import Path
from tempfile import NamedTemporaryFile

from .catalog import Product


def write_csv(products: Iterable[Product], path: str | Path) -> Path:
    """Write a UTF-8 CSV and replace the destination after a successful write."""
    destination = Path(path)
    with NamedTemporaryFile(
        mode="w", encoding="utf-8-sig", newline="", dir=destination.parent, delete=False
    ) as stream:
        temporary = Path(stream.name)
        try:
            writer = csv.writer(stream, delimiter=";")
            writer.writerow(
                (
                    "id",
                    "name",
                    "url",
                    "price_rub",
                    "old_price_rub",
                    "rating",
                    "review_count",
                    "images",
                )
            )
            for product in products:
                writer.writerow(
                    (
                        product.id,
                        product.name,
                        product.url,
                        product.price,
                        product.old_price,
                        product.rating,
                        product.review_count,
                        " | ".join(product.images),
                    )
                )
            stream.close()
            temporary.replace(destination)
        finally:
            stream.close()
            temporary.unlink(missing_ok=True)
    return destination
