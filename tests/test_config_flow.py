"""Tests for the motifini config flow."""

from __future__ import annotations

from unittest.mock import patch

from homeassistant.config_entries import SOURCE_USER
from homeassistant.const import CONF_HOST, CONF_PORT
from homeassistant.data_entry_flow import FlowResultType

from custom_components.motifini.client import MotifiniError, MotifiniResponseError
from custom_components.motifini.const import CONF_API_KEY, CONF_PATH_PREFIX, DOMAIN

from .conftest import ENTRY_DATA


def _patch_events(side_effect=None):
    return patch(
        "custom_components.motifini.client.MotifiniClient.async_events",
        return_value=[],
        side_effect=side_effect,
    )


def _patch_setup():
    return patch("custom_components.motifini.async_setup_entry", return_value=True)


async def _submit(hass, user_input):
    result = await hass.config_entries.flow.async_init(
        DOMAIN, context={"source": SOURCE_USER}
    )
    assert result["type"] is FlowResultType.FORM

    return await hass.config_entries.flow.async_configure(
        result["flow_id"], dict(user_input)
    )


async def test_user_flow_creates_entry(hass) -> None:
    """A reachable Motifini produces an entry keyed by host, port and prefix."""
    with _patch_events(), _patch_setup():
        result = await _submit(hass, ENTRY_DATA)
        await hass.async_block_till_done()

    assert result["type"] is FlowResultType.CREATE_ENTRY
    assert result["title"] == "Motifini motifini.lan:8765"
    assert result["result"].unique_id == "motifini.lan:8765/"
    assert result["data"][CONF_HOST] == "motifini.lan"


async def test_user_flow_brackets_ipv6_host(hass) -> None:
    """An IPv6 entry is titled and identified the way it is addressed."""
    with _patch_events(), _patch_setup():
        result = await _submit(hass, {**ENTRY_DATA, CONF_HOST: "fe80::1"})
        await hass.async_block_till_done()

    assert result["type"] is FlowResultType.CREATE_ENTRY
    assert result["title"] == "Motifini [fe80::1]:8765"
    assert result["result"].unique_id == "[fe80::1]:8765/"
    # The raw host is what the client is handed; it brackets on its own.
    assert result["data"][CONF_HOST] == "fe80::1"


async def test_user_flow_trims_host_and_prefix(hass) -> None:
    """Stray whitespace and slashes are cleaned before use."""
    with _patch_events(), _patch_setup():
        result = await _submit(
            hass,
            {**ENTRY_DATA, CONF_HOST: "  motifini.lan  ", CONF_PATH_PREFIX: "/mot/"},
        )
        await hass.async_block_till_done()

    assert result["data"][CONF_HOST] == "motifini.lan"
    assert result["data"][CONF_PATH_PREFIX] == "mot"
    assert result["result"].unique_id == "motifini.lan:8765/mot"


async def test_user_flow_maps_errors(hass) -> None:
    """Auth failures are told apart from everything else."""
    cases = [
        (MotifiniResponseError(401, "unauthorized"), "invalid_auth"),
        (MotifiniResponseError(403, "forbidden"), "invalid_auth"),
        (MotifiniResponseError(500, "boom"), "cannot_connect"),
        (MotifiniError("not JSON"), "cannot_connect"),
        (RuntimeError("surprise"), "unknown"),
    ]

    for side_effect, expected in cases:
        with _patch_events(side_effect=side_effect):
            result = await _submit(hass, ENTRY_DATA)

        assert result["type"] is FlowResultType.FORM
        assert result["errors"] == {"base": expected}, side_effect


async def test_user_flow_aborts_on_duplicate(hass, mock_config_entry) -> None:
    """The same host, port and prefix cannot be added twice."""
    mock_config_entry.add_to_hass(hass)

    with _patch_events(), _patch_setup():
        result = await _submit(hass, ENTRY_DATA)

    assert result["type"] is FlowResultType.ABORT
    assert result["reason"] == "already_configured"


async def test_user_flow_allows_a_second_server(hass, mock_config_entry) -> None:
    """A different port is a different Motifini."""
    mock_config_entry.add_to_hass(hass)

    with _patch_events(), _patch_setup():
        result = await _submit(hass, {**ENTRY_DATA, CONF_PORT: 8766})
        await hass.async_block_till_done()

    assert result["type"] is FlowResultType.CREATE_ENTRY
    assert result["result"].unique_id == "motifini.lan:8766/"


async def test_api_key_is_optional(hass) -> None:
    """An empty key is passed to the client as no key at all."""
    with (
        _patch_setup(),
        patch(
            "custom_components.motifini.config_flow.MotifiniClient",
            autospec=True,
        ) as client_cls,
    ):
        await _submit(hass, {**ENTRY_DATA, CONF_API_KEY: ""})
        await hass.async_block_till_done()

    assert client_cls.call_args.args[4] is None
