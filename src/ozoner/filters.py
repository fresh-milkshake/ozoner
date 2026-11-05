"""Read available filters and encode the storefront's query parameters."""

import json
from collections.abc import Iterator, Mapping, Sequence
from dataclasses import dataclass
from decimal import Decimal
from urllib.parse import parse_qsl, urlencode, urlsplit, urlunsplit

from .catalog import _object, _objects

FilterValue = str | int | Decimal | Sequence[str | int]


@dataclass(frozen=True, slots=True)
class FilterOption:
    """One selectable value advertised by the storefront."""

    value: str
    label: str
    selected: bool = False
    category_link: str | None = None


@dataclass(frozen=True, slots=True)
class StorefrontFilter:
    """A filter and the options or bounds returned for this listing."""

    key: str
    label: str
    kind: str
    options: tuple[FilterOption, ...] = ()
    minimum: str | None = None
    maximum: str | None = None


def with_params(path: str, overrides: Mapping[str, str]) -> str:
    """Replace named parameters while keeping other URL parameters and their order."""
    url = urlsplit(path)
    pairs = [
        pair
        for pair in parse_qsl(url.query, keep_blank_values=True)
        if pair[0] not in overrides
    ]
    return urlunsplit(url._replace(query=urlencode([*pairs, *overrides.items()])))


def listing_path(
    path: str, filters: Mapping[str, FilterValue] | None, sort: str | None
) -> str:
    """Encode scalar, range and multi-value filters in storefront format."""
    overrides: dict[str, str] = {}
    for key, value in (filters or {}).items():
        if not key or key in {
            "text",
            "sorting",
            "page",
            "layout_container",
            "layout_page_index",
        }:
            raise ValueError(f"invalid storefront filter key: {key!r}")
        if isinstance(value, (str, int, Decimal)):
            overrides[key] = str(value)
        else:
            overrides[key] = ",".join(map(str, value))
    if sort is not None:
        overrides["sorting"] = sort
    return with_params(path, overrides) if overrides else path


def _label(value: object) -> str:
    if isinstance(value, str):
        return value
    if isinstance(value, dict):
        for key in ("text", "title", "content"):
            if label := _label(value.get(key)):
                return label
        return _label(value.get("textRs"))
    if isinstance(value, list):
        return "".join(map(_label, value))
    return ""


def _options(spec: dict[str, object]) -> Iterator[FilterOption]:
    groups = [*_objects(spec.get("sections")), spec]
    for group in groups:
        for item in [*_objects(group.get("items")), *_objects(group.get("categories"))]:
            key = item.get("key")
            if isinstance(key, (str, int)):
                link = item.get("link")
                yield FilterOption(
                    str(key),
                    _label(item.get("title")) or str(key),
                    item.get("isSelected") is True,
                    link if isinstance(link, str) else None,
                )


def parse_filters(widgets: list[object]) -> list[StorefrontFilter]:
    """Decode filtersDesktop without hard-coding any category-specific keys."""
    result: dict[str, StorefrontFilter] = {}
    for widget in widgets:
        state = _object(json.loads(widget) if isinstance(widget, str) else widget)
        for section in _objects(state.get("sections")):
            for facet in _objects(section.get("filters")):
                kind, key = facet.get("type"), facet.get("key")
                if not isinstance(kind, str) or not isinstance(key, str):
                    continue
                spec = _object(facet.get(kind))
                bounds = _object(spec.get("rangeFilter")) or spec
                minimum, maximum = bounds.get("minValue"), bounds.get("maxValue")
                options = {option.value: option for option in _options(spec)}
                result[key] = StorefrontFilter(
                    key,
                    _label(spec.get("title")) or _label(bounds.get("title")) or key,
                    kind,
                    tuple(options.values()),
                    str(minimum) if minimum is not None else None,
                    str(maximum) if maximum is not None else None,
                )
    return list(result.values())
