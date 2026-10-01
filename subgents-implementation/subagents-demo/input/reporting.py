"""
reporting.py – Sample input module 3

Generates human-readable summary reports from an Inventory and a list of Orders.
"""

from __future__ import annotations

from datetime import datetime
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from inventory import Inventory
    from order_processor import Order


def inventory_summary(inventory: "Inventory") -> str:
    """
    Produce a plain-text summary of the current inventory state.

    Includes: total product count, total value, and low-stock warnings.
    """
    lines = [
        "=== Inventory Summary ===",
        f"Generated : {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}",
        f"Products  : {len(inventory)}",
        f"Total value: ${inventory.total_inventory_value():,.2f}",
    ]

    low_stock = inventory.low_stock_report()
    if low_stock:
        lines.append(f"\nLow-stock alerts ({len(low_stock)} product(s)):")
        for p in low_stock:
            lines.append(f"  • {p.name} (SKU: {p.sku})  qty={p.quantity}")

    return "\n".join(lines)


def orders_summary(orders: list["Order"]) -> str:
    """
    Produce a plain-text summary of a list of orders.

    Groups orders by status and reports aggregate revenue.
    """
    from order_processor import OrderStatus  # local import to avoid circularity

    by_status: dict[str, list["Order"]] = {}
    total_revenue = 0.0

    for order in orders:
        key = order.status.value
        by_status.setdefault(key, []).append(order)
        if order.status == OrderStatus.CONFIRMED:
            total_revenue += order.order_total

    lines = [
        "=== Orders Summary ===",
        f"Generated : {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}",
        f"Total orders: {len(orders)}",
        f"Revenue (confirmed): ${total_revenue:,.2f}",
        "",
    ]
    for status, group in sorted(by_status.items()):
        lines.append(f"  {status.upper()}: {len(group)} order(s)")

    return "\n".join(lines)


def full_report(inventory: "Inventory", orders: list["Order"]) -> str:
    """Combine inventory and orders summaries into one formatted report."""
    return (
        inventory_summary(inventory)
        + "\n\n"
        + orders_summary(orders)
    )
