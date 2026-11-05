"""Read structured product details from the public storefront."""

import json
import re
from dataclasses import dataclass
from decimal import Decimal
from urllib.parse import urlsplit

from scrapling import Selector

from .catalog import ORIGIN, Product, _number, _object, _objects

PRODUCT_PATH = re.compile(r"/product/(?:[^/]+-)?([0-9]+)/?")


@dataclass(frozen=True, slots=True)
class ProductDetails(Product):
    """Product details and the offer shown for the active storefront region."""

    brand: str | None = None
    description: str | None = None
    seller: str | None = None
    availability: str | None = None
    currency: str | None = None
    card_price: Decimal | None = None
    characteristics: tuple[tuple[str, str], ...] = ()


def product_path(reference: str | int | Product) -> tuple[str, str]:
    """Validate a product reference and return its canonical request path and ID."""
    value = reference.url if isinstance(reference, Product) else str(reference).strip()
    if value.isascii() and value.isdecimal():
        return f"/product/{value}/", value
    parsed = urlsplit(value)
    match = PRODUCT_PATH.fullmatch(parsed.path)
    if (
        not match
        or parsed.scheme != "https"
        or parsed.netloc not in {"ozon.ru", "www.ozon.ru"}
    ):
        raise ValueError(
            "expected an Ozon product ID or an https://www.ozon.ru/product/ URL"
        )
    return parsed.path.rstrip("/") + "/", match[1]


def _text(value: object) -> str | None:
    return value.strip() or None if isinstance(value, str) else None


def _state(document: Selector, widget: str) -> dict[str, object]:
    raw = document.css(f'[id^="state-{widget}-"]::attr(data-state)').get()
    return _object(json.loads(raw)) if raw else {}


def _characteristics(document: Selector) -> tuple[tuple[str, str], ...]:
    result: list[tuple[str, str]] = []
    for entry in _objects(
        _state(document, "webShortCharacteristics").get("characteristics")
    ):
        title = "".join(
            _text(part.get("content")) or ""
            for part in _objects(_object(entry.get("title")).get("textRs"))
        )
        values = ", ".join(
            text
            for part in _objects(entry.get("values"))
            if (text := _text(part.get("text")))
        )
        if title and values:
            result.append((title, values))
    return tuple(result)


def parse_details(html: str, expected_id: str) -> ProductDetails:
    """Decode Product JSON-LD and supplement it with visible offer widgets."""
    document = Selector(html)
    for node in document.css('script[type="application/ld+json"]'):
        try:
            data = _object(json.loads(node.text))
        except ValueError:
            continue
        if data.get("@type") != "Product" or str(data.get("sku")) != expected_id:
            continue
        if not (name := _text(data.get("name"))):
            continue
        offer = _object(data.get("offers"))
        rating = _object(data.get("aggregateRating"))
        prices = _state(document, "webPrice")
        seller = _object(_state(document, "webCurrentSeller").get("sellerCell"))
        canonical = document.css('link[rel="canonical"]::attr(href)').get()
        path, _ = product_path(canonical or expected_id)
        images = data.get("image")
        if isinstance(images, str):
            images = [images]
        return ProductDetails(
            id=expected_id,
            name=name,
            url=ORIGIN + path,
            price=_number(str(offer.get("price"))),
            old_price=_number(prices.get("originalPrice")),
            rating=_number(str(rating.get("ratingValue"))),
            review_count=int(count)
            if (count := _number(str(rating.get("reviewCount")))) is not None
            else None,
            images=tuple(
                image
                for image in images
                if isinstance(image, str) and image.startswith("https://")
            )
            if isinstance(images, list)
            else (),
            brand=_text(_object(data.get("brand")).get("name"))
            or _text(data.get("brand")),
            description=_text(data.get("description")),
            seller=_text(
                _object(_object(seller.get("centerBlock")).get("title")).get("text")
            ),
            availability=_text(offer.get("availability")),
            currency=_text(offer.get("priceCurrency")),
            card_price=_number(prices.get("cardPrice")),
            characteristics=_characteristics(document),
        )
    raise ValueError(f"no Product JSON-LD matched Ozon ID {expected_id}")
