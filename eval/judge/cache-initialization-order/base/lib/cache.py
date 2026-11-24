from .redis_client import RedisClient
from .settings import REDIS_URL

_client = RedisClient(REDIS_URL)


def get(key):
    return _client.get(key)
