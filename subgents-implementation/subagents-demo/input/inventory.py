"""
inventory.py – Sample input module 1

A minimal inventory management system used as the target codebase that the
sub-agents will explore and document.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional


@dataclass
class Product:
    """Represents a product stored in the warehouse."""

    sku: str
    name: str
    quantity: int = 0
    unit_price: float = 0.0

    def is_in_stock(self) -> bool:
        """Return True if there is at least one unit available."""
        return self.quantity > 0

    def total_value(self) -> float:
        """Return the total monetary value of all units in stock."""
        return self.quantity * self.unit_price


class Inventory:
    """
    In-memory inventory store.

    Manages a collection of Product objects and exposes common warehouse
    operations: add, remove, restock, and valuation.
    """

    def __init__(self) -> None:
        self._products: dict[str, Product] = {}

    def add_product(self, product: Product) -> None:
        """Register a new product; raises ValueError if SKU already exists."""
        if product.sku in self._products:
            raise ValueError(f"Product with SKU '{product.sku}' already exists.")
        self._products[product.sku] = product

    def remove_product(self, sku: str) -> Optional[Product]:
        """Remove and return a product by SKU, or None if not found."""
        return self._products.pop(sku, None)

    def restock(self, sku: str, quantity: int) -> None:
        """Increase the quantity of an existing product."""
        if sku not in self._products:
            raise KeyError(f"Unknown SKU: {sku}")
        if quantity <= 0:
            raise ValueError("Restock quantity must be positive.")
        self._products[sku].quantity += quantity

    def sell(self, sku: str, quantity: int) -> float:
        """
        Decrease product quantity and return the revenue generated.

        Raises ValueError if there is insufficient stock.
        """
        product = self._products.get(sku)
        if product is None:
            raise KeyError(f"Unknown SKU: {sku}")
        if product.quantity < quantity:
            raise ValueError(
                f"Insufficient stock for '{sku}': "
                f"have {product.quantity}, need {quantity}."
            )
        product.quantity -= quantity
        return quantity * product.unit_price

    def total_inventory_value(self) -> float:
        """Return the combined monetary value of all products."""
        return sum(p.total_value() for p in self._products.values())

    def low_stock_report(self, threshold: int = 5) -> list[Product]:
        """Return products whose quantity is at or below the threshold."""
        return [p for p in self._products.values() if p.quantity <= threshold]

    def __len__(self) -> int:
        return len(self._products)

    def __contains__(self, sku: str) -> bool:
        return sku in self._products
