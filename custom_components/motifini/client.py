"""Thin async HTTP client for the Motifini webserver event API."""

from __future__ import annotations

import asyncio
import json
from html import unescape
from typing import Any
from urllib.parse import quote

import aiohttp

REQUEST_TIMEOUT = 15


def url_host(host: str) -> str:
    """Return a host usable in a URL, bracketing an IPv6 literal.

    The config flow asks for a hostname or IP, and an IPv6 address has to be
    wrapped: http://fe80::1:8765 is not a parseable URL. A colon is the tell,
    since hostnames cannot contain one, and this keeps a zone id (fe80::1%en0)
    working too.
    """
    host = host.strip()
    if ":" in host and not host.startswith("["):
        return f"[{host}]"

    return host


class MotifiniError(Exception):
    """Base error for Motifini API failures."""


class MotifiniConnectionError(MotifiniError):
    """Raised when the Motifini server cannot be reached."""


class MotifiniResponseError(MotifiniError):
    """Raised when Motifini rejects a request with a 4xx/5xx status."""

    def __init__(self, status: int, message: str) -> None:
        """Store the HTTP status alongside the message."""
        super().__init__(f"Motifini returned {status}: {message}")
        self.status = status


class MotifiniClient:
    """Async client for the Motifini event API."""

    def __init__(
        self,
        session: aiohttp.ClientSession,
        host: str,
        port: int,
        path_prefix: str = "",
        api_key: str | None = None,
    ) -> None:
        """Initialize the client with a base URL and optional API key."""
        self._session = session
        self._headers = {"Authorization": f"Bearer {api_key}"} if api_key else None
        prefix = path_prefix.strip("/")
        self._base_url = f"http://{url_host(host)}:{port}"
        if prefix:
            self._base_url += f"/{prefix}"

    async def async_events(self) -> list[dict[str, Any]]:
        """Return the event catalog; also used as the connectivity check."""
        result = await self._request("GET", "/api/v1.0/events")
        try:
            events = json.loads(result)
        except json.JSONDecodeError as err:
            raise MotifiniError(
                "Unexpected reply from Motifini (not JSON); wrong host/port?"
            ) from err

        if not isinstance(events, list):
            raise MotifiniError("Unexpected reply from Motifini (not an event list)")

        return events

    async def async_register_event(self, event: str, description: str = "") -> str:
        """Register or update a catalog event."""
        return await self._request(
            "PUT",
            f"/api/v1.0/event/{quote(event, safe='')}",
            data={"description": description},
        )

    async def async_remove_event(self, event: str) -> str:
        """Remove a catalog event and every Telegram subscription for it."""
        return await self._request(
            "POST", f"/api/v1.0/event/remove/{quote(event, safe='')}"
        )

    async def async_notify(
        self,
        event: str,
        message: str | None = None,
        camera: str | None = None,
        media: str | None = None,
    ) -> str:
        """Send an event notification to the event's Telegram subscribers."""
        data: dict[str, str] = {}
        if message:
            data["msg"] = message
        if camera:
            data["camera"] = camera
        if media:
            data["media"] = media

        return await self._request(
            "POST", f"/api/v1.0/event/notify/{quote(event, safe='')}", data=data
        )

    async def _request(
        self,
        method: str,
        path: str,
        data: dict[str, str] | None = None,
    ) -> str:
        """Make a request and return the reply body, raising on failure."""
        try:
            # Release the response through its context manager rather than
            # relying on text() having read the body to the end, which is what
            # currently returns the connection to the pool. An early return
            # added above that read would otherwise strand a connection.
            async with (
                asyncio.timeout(REQUEST_TIMEOUT),
                self._session.request(
                    method, self._base_url + path, data=data, headers=self._headers
                ) as resp,
            ):
                status = resp.status
                body = await resp.text()
        except (aiohttp.ClientError, TimeoutError) as err:
            raise MotifiniConnectionError(f"Cannot reach Motifini: {err}") from err

        # Motifini HTML-escapes its plain-text replies; unescape for readability.
        body = unescape(body.strip())

        if status >= 400:
            raise MotifiniResponseError(status, body)

        return body
