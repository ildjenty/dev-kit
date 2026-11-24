import time
from .schedule import SUMMARY_REFRESH_SECONDS
from .summary import summarize


def refresh_forever(items, client, publish):
    cache = {}
    while True:
        publish(summarize(items, client, cache))
        time.sleep(SUMMARY_REFRESH_SECONDS)
