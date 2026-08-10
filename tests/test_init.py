"""Tests for motifini setup, unload, and the three services."""

from __future__ import annotations

from unittest.mock import patch

import pytest
from homeassistant.config_entries import ConfigEntryState
from homeassistant.const import CONF_PORT
from homeassistant.exceptions import HomeAssistantError
from pytest_homeassistant_custom_component.common import MockConfigEntry

from custom_components.motifini import ALL_SERVICES
from custom_components.motifini.client import MotifiniError
from custom_components.motifini.const import (
    DOMAIN,
    SERVICE_NOTIFY,
    SERVICE_REGISTER_EVENT,
    SERVICE_REMOVE_EVENT,
)

from .conftest import ENTRY_DATA

CLIENT = "custom_components.motifini.client.MotifiniClient"


def _patch_events(side_effect=None):
    return patch(f"{CLIENT}.async_events", return_value=[], side_effect=side_effect)


async def _setup(hass, entry: MockConfigEntry) -> None:
    entry.add_to_hass(hass)

    with _patch_events():
        assert await hass.config_entries.async_setup(entry.entry_id)
        await hass.async_block_till_done()


async def test_setup_registers_services(hass, mock_config_entry) -> None:
    """A loaded entry brings up all three services."""
    await _setup(hass, mock_config_entry)

    assert mock_config_entry.state is ConfigEntryState.LOADED
    for service in ALL_SERVICES:
        assert hass.services.has_service(DOMAIN, service), service


async def test_setup_retries_when_unreachable(hass, mock_config_entry) -> None:
    """An unreachable Motifini leaves the entry to retry, not fail outright."""
    mock_config_entry.add_to_hass(hass)

    with _patch_events(side_effect=MotifiniError("down")):
        await hass.config_entries.async_setup(mock_config_entry.entry_id)
        await hass.async_block_till_done()

    assert mock_config_entry.state is ConfigEntryState.SETUP_RETRY


async def test_services_dropped_with_the_last_entry(hass, mock_config_entry) -> None:
    """Services outlive one entry of two, and go with the last."""
    second = MockConfigEntry(
        domain=DOMAIN,
        title="Motifini motifini.lan:8766",
        data={**ENTRY_DATA, CONF_PORT: 8766},
        unique_id="motifini.lan:8766/",
    )

    await _setup(hass, mock_config_entry)
    await _setup(hass, second)

    assert await hass.config_entries.async_unload(second.entry_id)
    await hass.async_block_till_done()
    assert hass.services.has_service(DOMAIN, SERVICE_NOTIFY)

    assert await hass.config_entries.async_unload(mock_config_entry.entry_id)
    await hass.async_block_till_done()
    for service in ALL_SERVICES:
        assert not hass.services.has_service(DOMAIN, service), service


async def test_notify_passes_arguments_through(hass, mock_config_entry) -> None:
    """The service hands the call straight to the client."""
    await _setup(hass, mock_config_entry)

    with patch(f"{CLIENT}.async_notify", return_value="OK") as notify:
        await hass.services.async_call(
            DOMAIN,
            SERVICE_NOTIFY,
            {
                "event": "garage_opened",
                "message": "Garage opened",
                "camera": "Garage",
                "media": "photo",
            },
            blocking=True,
        )

    assert notify.await_args.args == ("garage_opened",)
    assert notify.await_args.kwargs == {
        "message": "Garage opened",
        "camera": "Garage",
        "media": "photo",
    }


@pytest.mark.parametrize(
    ("data", "match"),
    [
        ({"event": "evt", "media": "photo"}, "camera is required"),
        ({"event": "evt", "media": "video"}, "camera is required"),
        ({"event": "evt", "media": "none"}, "message is required"),
        ({"event": "evt"}, "Provide a message, or a camera"),
    ],
)
async def test_notify_rejects_bad_combinations(
    hass, mock_config_entry, data: dict[str, str], match: str
) -> None:
    """Combinations the server would reject fail before any request."""
    await _setup(hass, mock_config_entry)

    with (
        patch(f"{CLIENT}.async_notify") as notify,
        pytest.raises(HomeAssistantError, match=match),
    ):
        await hass.services.async_call(DOMAIN, SERVICE_NOTIFY, data, blocking=True)

    notify.assert_not_called()


async def test_notify_reports_client_failure(hass, mock_config_entry) -> None:
    """A Motifini error reaches the automation as a HomeAssistantError."""
    await _setup(hass, mock_config_entry)

    with (
        patch(f"{CLIENT}.async_notify", side_effect=MotifiniError("nope")),
        pytest.raises(HomeAssistantError, match="notify failed"),
    ):
        await hass.services.async_call(
            DOMAIN,
            SERVICE_NOTIFY,
            {"event": "evt", "message": "hi"},
            blocking=True,
        )


async def test_register_and_remove_event(hass, mock_config_entry) -> None:
    """register_event upserts a description; remove_event takes just a name."""
    await _setup(hass, mock_config_entry)

    with patch(f"{CLIENT}.async_register_event", return_value="OK") as register:
        await hass.services.async_call(
            DOMAIN,
            SERVICE_REGISTER_EVENT,
            {"event": "garage_opened", "description": "Garage door"},
            blocking=True,
        )

    assert register.await_args.args == ("garage_opened", "Garage door")

    with patch(f"{CLIENT}.async_remove_event", return_value="OK") as remove:
        await hass.services.async_call(
            DOMAIN,
            SERVICE_REMOVE_EVENT,
            {"event": "garage_opened"},
            blocking=True,
        )

    assert remove.await_args.args == ("garage_opened",)


async def test_two_entries_need_an_explicit_target(hass, mock_config_entry) -> None:
    """With two servers loaded, a call must say which one it means."""
    second = MockConfigEntry(
        domain=DOMAIN,
        title="Motifini motifini.lan:8766",
        data={**ENTRY_DATA, CONF_PORT: 8766},
        unique_id="motifini.lan:8766/",
    )
    await _setup(hass, mock_config_entry)
    await _setup(hass, second)

    with (
        patch(f"{CLIENT}.async_notify") as notify,
        pytest.raises(HomeAssistantError, match="pass config_entry_id"),
    ):
        await hass.services.async_call(
            DOMAIN, SERVICE_NOTIFY, {"event": "evt", "message": "hi"}, blocking=True
        )

    notify.assert_not_called()

    with patch(f"{CLIENT}.async_notify", return_value="OK") as notify:
        await hass.services.async_call(
            DOMAIN,
            SERVICE_NOTIFY,
            {
                "event": "evt",
                "message": "hi",
                "config_entry_id": second.entry_id,
            },
            blocking=True,
        )

    assert notify.await_count == 1
