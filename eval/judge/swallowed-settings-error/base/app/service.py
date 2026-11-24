from .settings import load_settings


def start(store):
    settings = load_settings(store)
    return {"enabled": settings.get("enabled", False)}
