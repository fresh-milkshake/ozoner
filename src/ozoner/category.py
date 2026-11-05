"""Identify Ozon category pages and expose their product listings."""

import re
from dataclasses import dataclass
from urllib.parse import urlsplit

from .catalog import ORIGIN, Product

CATEGORY_PATH = re.compile(r"/category/(?:[^/]+-)?([0-9]+)(?:/[^/]+)*/?")


@dataclass(frozen=True, slots=True)
class Category:
    """Category identity and the products collected from its storefront listing."""

    id: str
    name: str
    url: str
    products: tuple[Product, ...]


def category_path(reference: str) -> tuple[str, str]:
    """Accept an Ozon category URL and retain its storefront filters."""
    parsed = urlsplit(reference.strip())
    match = CATEGORY_PATH.fullmatch(parsed.path)
    if (
        not match
        or parsed.scheme != "https"
        or parsed.netloc not in {"ozon.ru", "www.ozon.ru"}
        or any(part in {".", ".."} for part in parsed.path.split("/"))
        or "\\" in parsed.path
    ):
        raise ValueError(f"expected a category URL under {ORIGIN}/category/")
    query = "?" + parsed.query if parsed.query else ""
    return parsed.path + query, match[1]
