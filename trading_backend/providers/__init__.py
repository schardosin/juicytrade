"""
Trading providers package.

This package contains the base provider interface and implementations
for different trading data providers (Alpaca, Public, etc.).
"""

from .base_provider import BaseProvider

__all__ = ["BaseProvider"]
