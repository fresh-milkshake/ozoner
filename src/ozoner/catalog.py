"""Decode Ozon catalog widgets into product summaries."""

import json
from collections.abc import Iterator
from dataclasses import dataclass
from decimal import Decimal, InvalidOperation
from urllib.parse import urlsplit

ORIGIN = "https://www.ozon.ru"


@dataclass(frozen=True, slots=True)
class Product:
    """One result from the public storefront in its current region."""

    id: str
    name: str
    url: str
    price: Decimal | None = None
    old_price: Decimal | None = None
    rating: Decimal | None = None
    review_count: int | None = None
    images: tuple[str, ...] = ()


def _number(value: object) -> Decimal | None:
    if not isinstance(value, str):
        return None
    try:
        number = Decimal("".join(value.removesuffix("₽").split()).replace(",", "."))
    except InvalidOperation:
        return None
    return number if number.is_finite() and number >= 0 else None


def _object(value: object) -> dict[str, object]:
    return value if isinstance(value, dict) else {}


def _objects(value: object) -> Iterator[dict[str, object]]:
    if isinstance(value, list):
        yield from (entry for entry in value if isinstance(entry, dict))


def _product(item: dict[str, object]) -> Product | None:
    identifier = str(item.get("id", ""))
    link = _object(item.get("action")).get("link")
    if (
        not identifier.isascii()
        or not identifier.isdecimal()
        or not isinstance(link, str)
    ):
        return None
    path = urlsplit(link)
    if path.scheme or path.netloc or not path.path.startswith("/product/"):
        return None
    if path.path.rstrip("/").split("/")[-1].rsplit("-", 1)[-1] != identifier:
        return None

    name = ""
    prices: dict[str, Decimal] = {}
    rating = None
    review_count = None
    for state in _objects(item.get("mainState")):
        if state.get("id") == "name":
            title = _object(state.get("textDS")).get("text")
            if isinstance(title, str):
                name = title.strip()
        for entry in _objects(_object(state.get("priceV2")).get("price")):
            style, number = entry.get("textStyle"), _number(entry.get("text"))
            if isinstance(style, str) and number is not None:
                prices.setdefault(style, number)
        labels = _object(state.get("labelListV2"))
        if (
            _object(labels.get("testInfo")).get("automatizationId")
            == "tile-list-rating"
        ):
            for entry in _objects(labels.get("items")):
                value = _object(entry.get("text")).get("text")
                if not isinstance(value, str):
                    continue
                number, separator, _ = value.partition("отзыв")
                parsed = _number(number)
                if parsed is not None and separator and parsed == int(parsed):
                    review_count = int(parsed)
                elif parsed is not None and not separator and 0 < parsed <= 5:
                    rating = parsed
    if not name:
        return None
    images = tuple(
        image
        for entry in _objects(_object(item.get("tileImage")).get("items"))
        if isinstance(image := _object(entry.get("image")).get("link"), str)
        and image.startswith("https://")
    )
    return Product(
        identifier,
        name,
        ORIGIN + path.path,
        prices.get("PRICE"),
        prices.get("ORIGINAL_PRICE"),
        rating,
        review_count,
        images,
    )


def parse_widgets(widgets: list[object]) -> Iterator[Product]:
    """Yield recognizable product tiles from embedded or API widget state."""
    for widget in widgets:
        state = json.loads(widget) if isinstance(widget, str) else widget
        for item in _objects(_object(state).get("items")):
            if (product := _product(item)) is not None:
                yield product
