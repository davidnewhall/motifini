"""Shared fixtures for motifini component tests."""

from __future__ import annotations

import asyncio
from typing import Any

import aiohttp
import pytest
import pytest_asyncio
import pytest_socket
from aiohttp import web
from homeassistant.const import CONF_HOST, CONF_PORT
from pytest_homeassistant_custom_component.common import MockConfigEntry
from pytest_homeassistant_custom_component.plugins import HASocketBlockedError

from custom_components.motifini.client import MotifiniClient
from custom_components.motifini.const import CONF_API_KEY, CONF_PATH_PREFIX, DOMAIN


# pytest-homeassistant-custom-component 0.13.x defines `enable_event_loop_debug`
# as a plain `@pytest.fixture(autouse=True)` async generator, which pytest 9
# rejects. Override it locally until the upstream plugin handles pytest 9.
@pytest_asyncio.fixture(autouse=True)
async def enable_event_loop_debug() -> None:
    """Enable event loop debug mode."""
    asyncio.get_running_loop().set_debug(True)


@pytest.fixture(autouse=True)
def auto_enable_custom_integrations(enable_custom_integrations: None) -> None:
    """Let the HA test harness discover custom_components/motifini."""
    return


ENTRY_DATA: dict[str, Any] = {
    CONF_HOST: "motifini.lan",
    CONF_PORT: 8765,
    CONF_API_KEY: "",
    CONF_PATH_PREFIX: "",
}


@pytest.fixture
def socket_enabled() -> None:
    """Let a test open loopback sockets.

    pytest-homeassistant-custom-component blocks socket creation outright and
    then asserts at teardown that nothing tried, so both have to be lifted for
    the tests that run a throwaway Motifini on a loopback port. Off-host
    connections stay blocked.
    """
    pytest_socket.enable_socket()
    pytest_socket.socket_allow_hosts(["127.0.0.1", "::1"])

    yield

    pytest_socket.socket_allow_hosts(["127.0.0.1"])
    pytest_socket.disable_socket(allow_unix_socket=True)
    HASocketBlockedError.instances.clear()


@pytest.fixture
def mock_config_entry() -> MockConfigEntry:
    """Config entry with typical user input."""
    return MockConfigEntry(
        domain=DOMAIN,
        title="Motifini motifini.lan:8765",
        data=ENTRY_DATA,
        unique_id="motifini.lan:8765/",
    )


class FakeMotifini:
    """A stand-in Motifini webserver that records what it was asked.

    The client is worth exercising against a real socket rather than a mock:
    URL quoting, form encoding and the Bearer header are the parts most likely
    to break, and none of them are visible when the transport is faked.
    """

    def __init__(self) -> None:
        """Start with an empty catalog and a 200 reply."""
        self.requests: list[dict[str, Any]] = []
        self.status = 200
        self.body = "OK: sent\n"
        self.events_body = '[{"name":"garage_opened","subscribers":2}]'
        self.port = 0

    async def handle(self, request: web.Request) -> web.Response:
        """Record one request and answer with the configured reply."""
        form = await request.post()
        self.requests.append(
            {
                "method": request.method,
                # raw_path keeps percent-encoding, which is the point of the
                # quoting in the client.
                "raw_path": request.raw_path,
                "form": {key: str(value) for key, value in form.items()},
                "auth": request.headers.get("Authorization"),
            }
        )

        if self.status >= 400:
            return web.Response(status=self.status, text=self.body)

        if request.method == "GET" and request.path.endswith("/api/v1.0/events"):
            return web.Response(text=self.events_body)

        return web.Response(status=self.status, text=self.body)

    @property
    def last(self) -> dict[str, Any]:
        """Return the most recent request."""
        return self.requests[-1]


@pytest_asyncio.fixture
async def fake_motifini(socket_enabled: None) -> FakeMotifini:
    """Run a FakeMotifini on localhost for the duration of a test."""
    fake = FakeMotifini()
    app = web.Application()
    app.router.add_route("*", "/{tail:.*}", fake.handle)

    runner = web.AppRunner(app)
    await runner.setup()
    site = web.TCPSite(runner, "127.0.0.1", 0)
    await site.start()
    fake.port = runner.addresses[0][1]

    yield fake

    await runner.cleanup()


@pytest_asyncio.fixture
async def session(socket_enabled: None) -> aiohttp.ClientSession:
    """Provide a client session that is closed with the test."""
    async with aiohttp.ClientSession() as client_session:
        yield client_session


def make_client(
    session: aiohttp.ClientSession,
    fake: FakeMotifini,
    path_prefix: str = "",
    api_key: str | None = None,
) -> MotifiniClient:
    """Build a client aimed at a running FakeMotifini."""
    return MotifiniClient(session, "127.0.0.1", fake.port, path_prefix, api_key)
