"""
order_processor.py – Sample input module 2

Handles order creation, validation, and fulfilment against an Inventory store.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from enum import Enum
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from inventory import Inventory


class OrderStatus(Enum):
    """Lifecycle states an order can be in."""
    PENDING = "pending"
    CONFIRMED = "confirmed"
    SHIPPED = "shipped"
    CANCELLED = "cancelled"


@dataclass
class OrderLine:
    """A single line within an order: one SKU and a requested quantity."""
    sku: str
    quantity: int
    unit_price: float = 0.0

    @property
    def line_total(self) -> float:
        return self.quantity * self.unit_price


@dataclass
class Order:
    """Represents a customer order composed of one or more order lines."""
    order_id: str
    customer_id: str
    lines: list[OrderLine] = field(default_factory=list)
    status: OrderStatus = OrderStatus.PENDING
    created_at: datetime = field(default_factory=datetime.now)

    def add_line(self, sku: str, quantity: int, unit_price: float) -> None:
        """Append a new line to the order."""
        self.lines.append(OrderLine(sku=sku, quantity=quantity, unit_price=unit_price))

    @property
    def order_total(self) -> float:
        """Return the sum of all line totals."""
        return sum(line.line_total for line in self.lines)

    def cancel(self) -> None:
        """Cancel the order if it has not yet shipped."""
        if self.status == OrderStatus.SHIPPED:
            raise RuntimeError("Cannot cancel a shipped order.")
        self.status = OrderStatus.CANCELLED


class OrderProcessor:
    """
    Validates and fulfils orders against an Inventory instance.

    Responsibilities:
      - Validate that all requested SKUs exist and have sufficient stock.
      - Deduct sold quantities from the inventory.
      - Transition order status through the lifecycle.
    """

    def __init__(self, inventory: "Inventory") -> None:
        self._inventory = inventory
        self._processed_orders: list[Order] = []

    def process(self, order: Order) -> float:
        """
        Validate and fulfil an order.

        Returns the total revenue collected.
        Raises ValueError on validation failure; the order status is unchanged.
        """
        if order.status != OrderStatus.PENDING:
            raise ValueError(
                f"Order {order.order_id} is not in PENDING state "
                f"(current: {order.status.value})."
            )

        # Validate all lines before touching inventory (all-or-nothing)
        for line in order.lines:
            if line.sku not in self._inventory:
                raise ValueError(f"Unknown SKU in order: {line.sku}")

        # Fulfil each line
        total_revenue = 0.0
        for line in order.lines:
            revenue = self._inventory.sell(line.sku, line.quantity)
            total_revenue += revenue

        order.status = OrderStatus.CONFIRMED
        self._processed_orders.append(order)
        return total_revenue

    def ship(self, order_id: str) -> None:
        """Mark a confirmed order as shipped."""
        for order in self._processed_orders:
            if order.order_id == order_id:
                if order.status != OrderStatus.CONFIRMED:
                    raise RuntimeError(
                        f"Order {order_id} cannot be shipped "
                        f"(status: {order.status.value})."
                    )
                order.status = OrderStatus.SHIPPED
                return
        raise KeyError(f"Order {order_id} not found in processed orders.")

    @property
    def processed_count(self) -> int:
        """Return the number of successfully processed orders."""
        return len(self._processed_orders)
