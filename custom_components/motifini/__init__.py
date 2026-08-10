"""The Motifini integration.

Connects Home Assistant to a Motifini daemon so automations can register
events and fire notifications (text, camera photo, or video clip) that
Telegram users subscribe to in the Motifini bot. No Telegram credentials
live in Home Assistant; Motifini owns the bot.
"""

from __future__ import annotations

import logging

import voluptuous as vol
from homeassistant.config_entries import ConfigEntry, ConfigEntryState
from homeassistant.const import CONF_HOST, CONF_PORT
from homeassistant.core import HomeAssistant, ServiceCall
from homeassistant.exceptions import ConfigEntryNotReady, HomeAssistantError
from homeassistant.helpers import config_validation as cv
from homeassistant.helpers.aiohttp_client import async_get_clientsession

from .client import MotifiniClient, MotifiniError
from .const import (
    ATTR_CAMERA,
    ATTR_DESCRIPTION,
    ATTR_EVENT,
    ATTR_MEDIA,
    ATTR_MESSAGE,
    CONF_API_KEY,
    CONF_PATH_PREFIX,
    DOMAIN,
    MEDIA_NONE,
    MEDIA_PHOTO,
    MEDIA_VIDEO,
    SERVICE_NOTIFY,
    SERVICE_REGISTER_EVENT,
    SERVICE_REMOVE_EVENT,
    VALID_MEDIA,
)

_LOGGER = logging.getLogger(__name__)

type MotifiniConfigEntry = ConfigEntry[MotifiniClient]

ALL_SERVICES = (SERVICE_NOTIFY, SERVICE_REGISTER_EVENT, SERVICE_REMOVE_EVENT)


async def async_setup_entry(hass: HomeAssistant, entry: MotifiniConfigEntry) -> bool:
    """Set up Motifini from a config entry."""
    session = async_get_clientsession(hass)
    client = MotifiniClient(
        session,
        entry.data[CONF_HOST],
        entry.data[CONF_PORT],
        entry.data.get(CONF_PATH_PREFIX, ""),
        entry.data.get(CONF_API_KEY) or None,
    )

    try:
        await client.async_events()
    except MotifiniError as err:
        raise ConfigEntryNotReady(f"Cannot connect to Motifini: {err}") from err

    entry.runtime_data = client
    _async_register_services(hass)
    return True


async def async_unload_entry(hass: HomeAssistant, entry: MotifiniConfigEntry) -> bool:
    """Unload a config entry; drop the services when the last one goes."""
    loaded = [
        e
        for e in hass.config_entries.async_entries(DOMAIN)
        if e.state is ConfigEntryState.LOADED and e.entry_id != entry.entry_id
    ]
    if not loaded:
        for service in ALL_SERVICES:
            hass.services.async_remove(DOMAIN, service)

    return True


def _async_register_services(hass: HomeAssistant) -> None:
    """Register domain services once."""
    if hass.services.has_service(DOMAIN, SERVICE_NOTIFY):
        return

    def _client_for_entry_id(entry_id: str | None) -> MotifiniClient:
        entries = [
            e
            for e in hass.config_entries.async_entries(DOMAIN)
            if e.state is ConfigEntryState.LOADED
            and (entry_id is None or e.entry_id == entry_id)
        ]
        if not entries:
            raise HomeAssistantError("No loaded Motifini config entry")
        if entry_id is None and len(entries) > 1:
            raise HomeAssistantError(
                "Multiple Motifini entries loaded; pass config_entry_id"
            )
        return entries[0].runtime_data

    async def handle_notify(call: ServiceCall) -> None:
        client = _client_for_entry_id(call.data.get("config_entry_id"))
        message = call.data.get(ATTR_MESSAGE)
        camera = call.data.get(ATTR_CAMERA)
        media = call.data.get(ATTR_MEDIA)

        # Mirror Motifini's combination rules so mistakes fail fast and clear.
        if media in (MEDIA_PHOTO, MEDIA_VIDEO) and not camera:
            raise HomeAssistantError(f"camera is required when media is {media}")
        if media == MEDIA_NONE and not message:
            raise HomeAssistantError("message is required when media is none")
        if media is None and not message and not camera:
            raise HomeAssistantError(
                "Provide a message, or a camera (media defaults to photo)"
            )

        try:
            await client.async_notify(
                call.data[ATTR_EVENT], message=message, camera=camera, media=media
            )
        except MotifiniError as err:
            raise HomeAssistantError(f"Motifini notify failed: {err}") from err

    async def handle_register_event(call: ServiceCall) -> None:
        client = _client_for_entry_id(call.data.get("config_entry_id"))
        try:
            await client.async_register_event(
                call.data[ATTR_EVENT], call.data.get(ATTR_DESCRIPTION, "")
            )
        except MotifiniError as err:
            raise HomeAssistantError(f"Motifini register_event failed: {err}") from err

    async def handle_remove_event(call: ServiceCall) -> None:
        client = _client_for_entry_id(call.data.get("config_entry_id"))
        try:
            await client.async_remove_event(call.data[ATTR_EVENT])
        except MotifiniError as err:
            raise HomeAssistantError(f"Motifini remove_event failed: {err}") from err

    hass.services.async_register(
        DOMAIN,
        SERVICE_NOTIFY,
        handle_notify,
        schema=vol.Schema(
            {
                vol.Required(ATTR_EVENT): cv.string,
                vol.Optional(ATTR_MESSAGE): cv.string,
                vol.Optional(ATTR_CAMERA): cv.string,
                vol.Optional(ATTR_MEDIA): vol.In(VALID_MEDIA),
                vol.Optional("config_entry_id"): cv.string,
            }
        ),
    )
    hass.services.async_register(
        DOMAIN,
        SERVICE_REGISTER_EVENT,
        handle_register_event,
        schema=vol.Schema(
            {
                vol.Required(ATTR_EVENT): cv.string,
                vol.Optional(ATTR_DESCRIPTION): cv.string,
                vol.Optional("config_entry_id"): cv.string,
            }
        ),
    )
    hass.services.async_register(
        DOMAIN,
        SERVICE_REMOVE_EVENT,
        handle_remove_event,
        schema=vol.Schema(
            {
                vol.Required(ATTR_EVENT): cv.string,
                vol.Optional("config_entry_id"): cv.string,
            }
        ),
    )
