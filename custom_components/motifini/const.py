"""Constants for the Motifini integration."""

DOMAIN = "motifini"

CONF_PATH_PREFIX = "path_prefix"
CONF_API_KEY = "api_key"

DEFAULT_PORT = 8765

SERVICE_NOTIFY = "notify"
SERVICE_REGISTER_EVENT = "register_event"
SERVICE_REMOVE_EVENT = "remove_event"

ATTR_EVENT = "event"
ATTR_MESSAGE = "message"
ATTR_CAMERA = "camera"
ATTR_MEDIA = "media"
ATTR_DESCRIPTION = "description"

MEDIA_NONE = "none"
MEDIA_PHOTO = "photo"
MEDIA_VIDEO = "video"
VALID_MEDIA = (MEDIA_NONE, MEDIA_PHOTO, MEDIA_VIDEO)
