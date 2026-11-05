"""Regression checks for storefront query encoding and facet parsing."""

import json
import unittest
from urllib.parse import parse_qsl, urlsplit

from ozoner.filters import listing_path, parse_filters


class FilterTests(unittest.TestCase):
    """Cover encoded ranges, selected options and category filter overrides."""

    def test_listing_overrides_preserve_other_parameters(self) -> None:
        """Existing filters survive unless explicitly replaced by the caller."""
        path = listing_path(
            "/category/phones-1/?text=A%26B&brand=3&flag=1&flag=2",
            {"brand": [1, 2], "price": "100;200"},
            "price",
        )
        self.assertEqual(
            parse_qsl(urlsplit(path).query),
            [
                ("text", "A&B"),
                ("flag", "1"),
                ("flag", "2"),
                ("brand", "1,2"),
                ("price", "100;200"),
                ("sorting", "price"),
            ],
        )

    def test_parse_site_options_and_range(self) -> None:
        """Values retain site IDs, labels, selections and range limits."""
        widget = {
            "sections": [
                {
                    "filters": [
                        {
                            "key": "brand",
                            "type": "list",
                            "list": {
                                "sections": [
                                    {
                                        "items": [
                                            {
                                                "key": 7,
                                                "title": {"text": "Brand"},
                                                "isSelected": True,
                                            }
                                        ],
                                    }
                                ],
                            },
                        },
                        {
                            "key": "price",
                            "type": "range",
                            "range": {
                                "rangeFilter": {"minValue": 100, "maxValue": 200}
                            },
                        },
                    ],
                },
            ],
        }
        brand, price = parse_filters([json.dumps(widget)])
        self.assertEqual(
            (brand.options[0].value, brand.options[0].label), ("7", "Brand")
        )
        self.assertTrue(brand.options[0].selected)
        self.assertEqual((price.minimum, price.maximum), ("100", "200"))
