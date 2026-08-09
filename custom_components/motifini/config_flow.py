"""Config flow for the Motifini integration."""

from __future__ import annotations

import logging
from typing import Any

import voluptuous as vol
from homeassistant import config_entries
from homeassistant.config_entries import ConfigFlowResult
from homeassistant.const import CONF_HOST, CONF_PORT
from homeassistant.core import HomeAssistant
from homeassistant.helpers.aiohttp_client import async_get_clientsession
from homeassistant.helpers.selector import (
    TextSelector,
    TextSelectorConfig,
    TextSelectorType,
)

from .client import MotifiniClient, MotifiniError, MotifiniResponseError
from .const import CONF_API_KEY, CONF_PATH_PREFIX, DEFAULT_PORT, DOMAIN

_LOGGER = logging.getLogger(__name__)

STEP_USER_DATA_SCHEMA = vol.Schema(
    {
        vol.Required(CONF_HOST): str,
        vol.Required(CONF_PORT, default=DEFAULT_PORT): vol.All(
            vol.Coerce(int), vol.Range(min=1, max=65535)
        ),
        vol.Optional(CONF_API_KEY, default=""): TextSelector(
            TextSelectorConfig(type=TextSelectorType.PASSWORD)
        ),
        vol.Optional(CONF_PATH_PREFIX, default=""): str,
    }
)


async def _validate_input(hass: HomeAssistant, data: dict[str, Any]) -> None:
    """Check that Motifini is reachable and answers the events API."""
    session = async_get_clientsession(hass)
    client = MotifiniClient(
        session,
        data[CONF_HOST],
        data[CONF_PORT],
        data[CONF_PATH_PREFIX],
        data.get(CONF_API_KEY) or None,
    )
    await client.async_events()


class MotifiniConfigFlow(config_entries.ConfigFlow, domain=DOMAIN):
    """Handle a config flow for Motifini."""

    VERSION = 1

    async def async_step_user(
        self, user_input: dict[str, Any] | None = None
    ) -> ConfigFlowResult:
        """Handle the initial step."""
        errors: dict[str, str] = {}
        if user_input is not None:
            user_input[CONF_HOST] = user_input[CONF_HOST].strip()
            user_input[CONF_PATH_PREFIX] = user_input[CONF_PATH_PREFIX].strip("/")
            try:
                await _validate_input(self.hass, user_input)
            except MotifiniResponseError as err:
                errors["base"] = (
                    "invalid_auth" if err.status in (401, 403) else "cannot_connect"
                )
            except MotifiniError:
                errors["base"] = "cannot_connect"
            except Exception:
                _LOGGER.exception("Unexpected error validating Motifini")
                errors["base"] = "unknown"
            else:
                await self.async_set_unique_id(
                    f"{user_input[CONF_HOST]}:{user_input[CONF_PORT]}"
                    f"/{user_input[CONF_PATH_PREFIX]}"
                )
                self._abort_if_unique_id_configured()
                return self.async_create_entry(
                    title=f"Motifini {user_input[CONF_HOST]}:{user_input[CONF_PORT]}",
                    data=user_input,
                )

        return self.async_show_form(
            step_id="user", data_schema=STEP_USER_DATA_SCHEMA, errors=errors
        )
