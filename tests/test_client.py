"""Tests for the motifini HTTP client."""

from __future__ import annotations

import asyncio
import socket

import aiohttp
import pytest
from aiohttp import web

from custom_components.motifini.client import (
    MotifiniClient,
    MotifiniConnectionError,
    MotifiniError,
    MotifiniResponseError,
    url_host,
)

from .conftest import FakeMotifini, make_client


@pytest.mark.parametrize(
    ("host", "expected"),
    [
        ("127.0.0.1", "127.0.0.1"),
        ("motifini.lan", "motifini.lan"),
        ("  10.0.0.5  ", "10.0.0.5"),
        # An IPv6 literal has to be bracketed: http://fe80::1:8765 is not a URL.
        ("::1", "[::1]"),
        ("fe80::1", "[fe80::1]"),
        # A zone id must survive, which rules out ipaddress parsing.
        ("fe80::1%en0", "[fe80::1%en0]"),
        ("[fe80::1]", "[fe80::1]"),
    ],
)
def test_url_host(host: str, expected: str) -> None:
    """Hosts are formatted for use in a URL."""
    assert url_host(host) == expected


def test_base_url_includes_prefix_and_brackets(session: aiohttp.ClientSession) -> None:
    """The base URL carries the reverse-proxy prefix and brackets IPv6."""
    plain = MotifiniClient(session, "motifini.lan", 8765)
    assert plain._base_url == "http://motifini.lan:8765"

    prefixed = MotifiniClient(session, "motifini.lan", 8765, "/motifini/")
    assert prefixed._base_url == "http://motifini.lan:8765/motifini"

    six = MotifiniClient(session, "fe80::1", 8765, "motifini")
    assert six._base_url == "http://[fe80::1]:8765/motifini"


async def test_events_returns_catalog(
    session: aiohttp.ClientSession, fake_motifini: FakeMotifini
) -> None:
    """The events call parses the JSON catalog."""
    client = make_client(session, fake_motifini)

    assert await client.async_events() == [{"name": "garage_opened", "subscribers": 2}]
    assert fake_motifini.last["method"] == "GET"
    assert fake_motifini.last["raw_path"] == "/api/v1.0/events"


async def test_events_rejects_non_json(
    session: aiohttp.ClientSession, fake_motifini: FakeMotifini
) -> None:
    """A non-JSON reply means the host/port points at something else."""
    fake_motifini.events_body = "<html>not motifini</html>"
    client = make_client(session, fake_motifini)

    with pytest.raises(MotifiniError, match="not JSON"):
        await client.async_events()


async def test_events_rejects_non_list(
    session: aiohttp.ClientSession, fake_motifini: FakeMotifini
) -> None:
    """Valid JSON that is not a list is still the wrong server."""
    fake_motifini.events_body = '{"name":"garage"}'
    client = make_client(session, fake_motifini)

    with pytest.raises(MotifiniError, match="not an event list"):
        await client.async_events()


async def test_notify_sends_form_fields(
    session: aiohttp.ClientSession, fake_motifini: FakeMotifini
) -> None:
    """Only the fields that were given are sent."""
    client = make_client(session, fake_motifini)

    await client.async_notify(
        "garage_opened", message="Garage opened", camera="Garage", media="photo"
    )

    assert fake_motifini.last["method"] == "POST"
    assert fake_motifini.last["raw_path"] == "/api/v1.0/event/notify/garage_opened"
    assert fake_motifini.last["form"] == {
        "msg": "Garage opened",
        "camera": "Garage",
        "media": "photo",
    }

    await client.async_notify("garage_opened", message="Just text")
    assert fake_motifini.last["form"] == {"msg": "Just text"}


async def test_event_names_are_quoted(
    session: aiohttp.ClientSession, fake_motifini: FakeMotifini
) -> None:
    """An event name with a slash or space must not reshape the path."""
    client = make_client(session, fake_motifini)

    await client.async_notify("front/back door", message="hi")
    assert fake_motifini.last["raw_path"] == (
        "/api/v1.0/event/notify/front%2Fback%20door"
    )

    await client.async_register_event("front/back door", "Both doors")
    assert fake_motifini.last["method"] == "PUT"
    assert fake_motifini.last["raw_path"] == "/api/v1.0/event/front%2Fback%20door"
    assert fake_motifini.last["form"] == {"description": "Both doors"}

    await client.async_remove_event("front/back door")
    assert fake_motifini.last["method"] == "POST"
    assert fake_motifini.last["raw_path"] == (
        "/api/v1.0/event/remove/front%2Fback%20door"
    )


async def test_path_prefix_is_applied(
    session: aiohttp.ClientSession, fake_motifini: FakeMotifini
) -> None:
    """A reverse-proxy prefix lands in front of the API path."""
    client = make_client(session, fake_motifini, path_prefix="/motifini/")

    await client.async_remove_event("garage_opened")

    assert fake_motifini.last["raw_path"] == (
        "/motifini/api/v1.0/event/remove/garage_opened"
    )


async def test_api_key_sent_as_bearer(
    session: aiohttp.ClientSession, fake_motifini: FakeMotifini
) -> None:
    """The key travels in a header, never in the query string."""
    keyed = make_client(session, fake_motifini, api_key="s3cret")
    await keyed.async_events()
    assert fake_motifini.last["auth"] == "Bearer s3cret"
    assert "s3cret" not in fake_motifini.last["raw_path"]

    plain = make_client(session, fake_motifini)
    await plain.async_events()
    assert fake_motifini.last["auth"] is None


async def test_error_status_raises_with_unescaped_body(
    session: aiohttp.ClientSession, fake_motifini: FakeMotifini
) -> None:
    """Motifini HTML-escapes its replies; the error message should not."""
    fake_motifini.status = 400
    fake_motifini.body = "ERROR: camera not found: Jim&#39;s Office\n"
    client = make_client(session, fake_motifini)

    with pytest.raises(MotifiniResponseError) as err:
        await client.async_notify("evt", camera="Jim's Office", media="photo")

    assert err.value.status == 400
    assert "Jim's Office" in str(err.value)


async def test_unauthorized_status_is_reported(
    session: aiohttp.ClientSession, fake_motifini: FakeMotifini
) -> None:
    """A 401 must surface its status so the config flow can map it."""
    fake_motifini.status = 401
    fake_motifini.body = "ERROR: unauthorized (bad or missing api key)\n"
    client = make_client(session, fake_motifini)

    with pytest.raises(MotifiniResponseError) as err:
        await client.async_events()

    assert err.value.status == 401


async def test_unreachable_server_raises_connection_error(
    session: aiohttp.ClientSession,
) -> None:
    """Nothing listening is a connection error, not a response error."""
    # Claim a port, then let it go, so the connect is refused rather than slow.
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]

    client = MotifiniClient(session, "127.0.0.1", port)

    with pytest.raises(MotifiniConnectionError, match="Cannot reach Motifini"):
        await client.async_events()


async def test_ipv6_host_is_reachable(socket_enabled: None) -> None:
    """A client aimed at an IPv6 literal must reach the server."""

    async def events(request: web.Request) -> web.Response:
        return web.Response(text='[{"name":"evt"}]')

    app = web.Application()
    app.router.add_get("/api/v1.0/events", events)
    runner = web.AppRunner(app)
    await runner.setup()
    site = web.TCPSite(runner, "::1", 0)
    await site.start()
    port = runner.addresses[0][1]

    try:
        async with aiohttp.ClientSession() as session:
            client = MotifiniClient(session, "::1", port)
            assert await client.async_events() == [{"name": "evt"}]
    finally:
        await runner.cleanup()


async def test_error_replies_do_not_strand_connections(
    fake_motifini: FakeMotifini,
) -> None:
    """Every request returns its connection, error replies included.

    A one-connection pool makes a stranded connection obvious: the next request
    would wait on the pool instead of being served.
    """
    fake_motifini.status = 400
    fake_motifini.body = "ERROR: nope\n"

    connector = aiohttp.TCPConnector(limit=1)
    async with aiohttp.ClientSession(connector=connector) as session:
        client = make_client(session, fake_motifini)

        for _ in range(3):
            with pytest.raises(MotifiniResponseError):
                await asyncio.wait_for(client.async_notify("evt", message="hi"), 5)

        assert not connector._acquired

        fake_motifini.status = 200
        assert await asyncio.wait_for(client.async_events(), 5)
