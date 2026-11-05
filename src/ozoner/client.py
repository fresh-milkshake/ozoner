"""Fetch the storefront in an isolated Scrapling browser session."""

import logging
import time
from collections.abc import Callable, Iterable, Mapping
from decimal import Decimal
from typing import TypeVar
from urllib.parse import urlencode, urlsplit

from patchright.sync_api import Error as BrowserError
from patchright.sync_api import Page
from scrapling.fetchers import StealthyFetcher

from .catalog import ORIGIN, Product, parse_widgets
from .category import Category, category_path
from .details import ProductDetails, parse_details, product_path
from .filters import (
    FilterValue,
    StorefrontFilter,
    listing_path,
    parse_filters,
    with_params,
)

logger = logging.getLogger(__name__)
T = TypeVar("T")

GRID = "[id^='state-tileGridDesktop-'][data-state]"
INITIAL = """
(selector) => {
    const grids = [...document.querySelectorAll(selector)];
    const paginator = document.querySelector("[id^='state-infiniteVirtualPaginator-'][data-state]");
    return {
        widgets: grids.map(node => node.dataset.state),
        next: paginator ? JSON.parse(paginator.dataset.state).nextPage : null
    };
}
"""
NEXT = """
async ({path, timeout, widget}) => {
    const query = new URLSearchParams({url: path});
    const response = await fetch("/api/entrypoint-api.bx/page/json/v2?" + query, {
        signal: AbortSignal.timeout(timeout)
    });
    if (!response.ok) return {status: response.status};
    const data = await response.json();
    return {
        status: response.status,
        widgets: Object.entries(data.widgetStates || {})
            .filter(([key]) => key.startsWith(widget + "-"))
            .map(([, value]) => value),
        next: data.nextPage
    };
}
"""


class OzonError(RuntimeError):
    """The storefront or browser did not return readable catalog data."""


class OzonHTTPError(OzonError):
    """The storefront returned an unsuccessful HTTP status."""

    def __init__(self, status: int, operation: str) -> None:
        """Record the status without including request URLs or session data."""
        self.status = status
        super().__init__(f"Ozon {operation} returned HTTP {status}")


class OzonClient:
    """Search the public catalog using one temporary headless Chrome per call."""

    def __init__(self, *, chrome_path: str | None = None, timeout: float = 25) -> None:
        """Configure Chrome and the timeout in seconds for each browser operation."""
        if not 0.001 <= timeout <= 300:
            raise ValueError("timeout must be between 0.001 and 300 seconds")
        self.chrome_path = chrome_path
        self.timeout_ms = int(timeout * 1000)

    def search(
        self,
        query: str,
        *,
        limit: int = 40,
        max_pages: int = 10,
        min_price: Decimal | int | None = None,
        max_price: Decimal | int | None = None,
        filters: Mapping[str, FilterValue] | None = None,
        sort: str | None = None,
    ) -> list[Product]:
        """Return up to limit unique products, in storefront order.

        A page cap or the end of the catalog can produce fewer results.
        An unreadable page or failed pagination raises OzonError.
        """
        query = query.strip()
        if not query or limit < 1 or max_pages < 1:
            raise ValueError("query must be nonempty and limits must be positive")
        if any(
            bound is not None and (not Decimal(bound).is_finite() or bound < 0)
            for bound in (min_price, max_price)
        ):
            raise ValueError("price bounds must be finite and nonnegative")
        if min_price is not None and max_price is not None and min_price > max_price:
            raise ValueError("min_price must not exceed max_price")

        return self._fetch(
            "search",
            listing_path(f"/search/?{urlencode({'text': query})}", filters, sort),
            lambda page: self._collect(page, limit, max_pages, min_price, max_price),
        )

    def get_product(self, reference: str | int | Product) -> ProductDetails:
        """Get the detail page for an Ozon ID, product URL or search result."""
        return self.get_products([reference])[0]

    def get_category(
        self,
        url: str,
        *,
        limit: int = 40,
        max_pages: int = 10,
        filters: Mapping[str, FilterValue] | None = None,
        sort: str | None = None,
    ) -> Category:
        """Read the heading and product listing for a public Ozon category URL."""
        path, _ = category_path(url)
        if limit < 1 or max_pages < 1:
            raise ValueError("limits must be positive")

        def collect_category(page: Page) -> list[Category]:
            final_path, identifier = category_path(page.url)
            heading = page.locator("h1").first
            name = (heading.text_content() or "").strip() if heading.count() else ""
            products = self._collect(page, limit, max_pages, None, None)
            return [
                Category(
                    identifier,
                    name or page.title(),
                    ORIGIN + urlsplit(final_path).path,
                    tuple(products),
                )
            ]

        return self._fetch(
            "category", listing_path(path, filters, sort), collect_category
        )[0]

    def get_filters(
        self,
        query: str | None = None,
        *,
        category_url: str | None = None,
        filters: Mapping[str, FilterValue] | None = None,
    ) -> list[StorefrontFilter]:
        """Discover the filter groups Ozon exposes for a search or category."""
        if (query is None) == (category_url is None):
            raise ValueError("provide either query or category_url")
        if category_url is not None:
            path, _ = category_path(category_url)
        else:
            if not query or not query.strip():
                raise ValueError("query must be nonempty")
            path = f"/search/?{urlencode({'text': query.strip()})}"

        def collect_filters(page: Page) -> list[StorefrontFilter]:
            embedded = page.locator(
                "[id^='state-filtersDesktop-'][data-state]"
            ).evaluate_all("nodes => nodes.map(node => node.dataset.state)")
            if facets := parse_filters(embedded):
                return facets
            url = urlsplit(page.url)
            layout = with_params(
                url.path + ("?" + url.query if url.query else ""),
                {"layout_container": "filtersDesktop", "layout_page_index": "2"},
            )
            batch = page.evaluate(
                NEXT,
                {
                    "path": layout,
                    "timeout": self.timeout_ms,
                    "widget": "filtersDesktop",
                },
            )
            if batch["status"] != 200:
                raise OzonHTTPError(batch["status"], "filters")
            if not batch["widgets"]:
                raise OzonError("Ozon returned no storefront filter widget")
            return parse_filters(batch["widgets"])

        return self._fetch(
            "filters", listing_path(path, filters, None), collect_filters
        )

    def get_products(
        self, references: Iterable[str | int | Product]
    ) -> list[ProductDetails]:
        """Fetch unique products in input order, reusing one browser tab.

        Raises OzonError on the first failed product instead of returning a partial batch.
        """
        paths: dict[str, str] = {}
        for reference in references:
            path, identifier = product_path(reference)
            paths.setdefault(identifier, path)
        if not paths:
            return []

        def collect_details(page: Page) -> list[ProductDetails]:
            products = []
            for identifier, path in paths.items():
                time.sleep(1)
                response = page.goto(
                    ORIGIN + path, wait_until="domcontentloaded", referer=page.url
                )
                if response is None:
                    raise OzonError(f"no response for product {identifier}")
                if response.status != 200:
                    raise OzonHTTPError(response.status, f"product {identifier}")
                products.append(parse_details(page.content(), identifier))
                logger.debug(
                    "product loaded id=%s status=%d", identifier, response.status
                )
            return products

        return self._fetch(
            "product lookup",
            f"/search/?{urlencode({'text': 'техника'})}",
            collect_details,
        )

    def _fetch(
        self, operation: str, path: str, read: Callable[[Page], list[T]]
    ) -> list[T]:
        started = time.monotonic()
        logger.info("%s started", operation)
        products: list[T] | None = None
        failure: Exception | None = None

        def collect(page: Page) -> None:
            """Collect inside the tab while its session and cookies are active."""
            nonlocal products, failure
            try:
                if page.url.startswith(ORIGIN + "/"):
                    products = read(page)
            except (BrowserError, OzonError, ValueError) as exc:
                failure = exc

        options = {"executable_path": self.chrome_path} if self.chrome_path else {}
        try:
            response = StealthyFetcher.fetch(
                ORIGIN + path,
                headless=True,
                real_chrome=True,
                google_search=False,
                solve_cloudflare=False,
                retries=1,
                timeout=self.timeout_ms,
                page_action=collect,
                **options,
            )
        except BrowserError as exc:
            logger.warning("%s failed error=%s", operation, type(exc).__name__)
            raise OzonError(
                f"Chrome {operation} failed ({type(exc).__name__})"
            ) from exc
        if failure is not None:
            if isinstance(failure, OzonHTTPError):
                logger.warning("%s failed status=%d", operation, failure.status)
                raise failure
        if response.status != 200:
            logger.warning("%s failed status=%d", operation, response.status)
            raise OzonHTTPError(response.status, operation)
        if failure is not None:
            logger.warning("%s failed error=%s", operation, type(failure).__name__)
            if isinstance(failure, OzonError):
                raise failure
            raise OzonError(
                f"Could not read Ozon {operation} ({type(failure).__name__})"
            ) from failure
        if products is None:
            raise OzonError("Scrapling did not return catalog data")
        logger.info(
            "%s complete status=%d products=%d elapsed_ms=%d",
            operation,
            response.status,
            len(products),
            (time.monotonic() - started) * 1000,
        )
        return products

    def _collect(
        self,
        page: Page,
        limit: int,
        max_pages: int,
        min_price: Decimal | int | None,
        max_price: Decimal | int | None,
    ) -> list[Product]:
        page.locator(GRID).first.wait_for(state="attached", timeout=self.timeout_ms)
        batch = page.evaluate(INITIAL, GRID)
        products: dict[str, Product] = {}
        visited: set[str] = set()

        for page_index in range(max_pages):
            logger.debug(
                "catalog page=%d widgets=%d", page_index + 1, len(batch["widgets"])
            )
            for product in parse_widgets(batch["widgets"]):
                if min_price is not None and (
                    product.price is None or product.price < min_price
                ):
                    continue
                if max_price is not None and (
                    product.price is None or product.price > max_price
                ):
                    continue
                products.setdefault(product.id, product)
                if len(products) == limit:
                    return list(products.values())
            path = batch["next"]
            if not path or page_index + 1 == max_pages:
                break
            if (
                not isinstance(path, str)
                or not path.startswith("/")
                or path.startswith("//")
            ):
                raise OzonError("Ozon returned an invalid pagination path")
            if urlsplit(path).netloc or path in visited:
                raise OzonError("Ozon returned an external or repeated pagination path")
            visited.add(path)
            time.sleep(1)
            batch = page.evaluate(
                NEXT,
                {"path": path, "timeout": self.timeout_ms, "widget": "tileGridDesktop"},
            )
            if batch["status"] != 200:
                raise OzonHTTPError(batch["status"], "pagination")
            if not batch["widgets"]:
                raise OzonError("Ozon pagination returned no catalog widget")

        return list(products.values())
