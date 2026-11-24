class RedisClient:
    def __init__(self, url):
        self.url = url
        self.values = {}

    def get(self, key):
        return self.values.get(key)
