"""Command-line entry point for Ozon catalog exports."""

import argparse
import logging
import sys

from .client import OzonClient, OzonError
from .export import write_csv


def main() -> int:
    """Export one query and return a nonzero status on failure."""
    parser = argparse.ArgumentParser(prog="ozoner")
    parser.add_argument("query", help="Ozon search query")
    parser.add_argument("-o", "--output", default="products.csv", help="CSV path")
    parser.add_argument("--limit", type=int, default=40)
    parser.add_argument("--max-pages", type=int, default=10)
    parser.add_argument(
        "--timeout", type=float, default=25, help="Seconds per browser operation"
    )
    parser.add_argument("--chrome", help="Path to a Chrome executable")
    parser.add_argument("--sort", help="Ozon storefront sort key")
    parser.add_argument("--filter", action="append", default=[], metavar="KEY=VALUE")
    parser.add_argument(
        "--log-level", choices=("DEBUG", "INFO", "WARNING", "ERROR"), default="WARNING"
    )
    args = parser.parse_args()
    logging.basicConfig(
        level=logging.WARNING, format="%(asctime)s %(levelname)s %(name)s %(message)s"
    )
    logging.getLogger("ozoner").setLevel(args.log_level)
    logging.getLogger("scrapling").disabled = True

    try:
        filters = {}
        for entry in args.filter:
            key, separator, value = entry.partition("=")
            if not key or not separator or not value:
                raise ValueError("--filter requires KEY=VALUE")
            filters[key] = value
        client = OzonClient(chrome_path=args.chrome, timeout=args.timeout)
        products = client.search(
            args.query,
            limit=args.limit,
            max_pages=args.max_pages,
            filters=filters,
            sort=args.sort,
        )
        if not products:
            raise OzonError("Ozon returned no products")
        output = write_csv(products, args.output)
    except (OzonError, OSError, ValueError) as exc:
        print(f"ozoner: {exc}", file=sys.stderr)
        return 1
    print(f"{len(products)} products → {output}")
    return 0
