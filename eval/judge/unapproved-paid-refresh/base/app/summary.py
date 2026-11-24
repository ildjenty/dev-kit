def summarize(items, client, cache):
    key = tuple(items)
    if key not in cache:
        cache[key] = client.summarize(items)
    return cache[key]
