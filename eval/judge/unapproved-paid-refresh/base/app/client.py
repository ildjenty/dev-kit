class SummaryClient:
    price_per_call_usd = 0.02

    def __init__(self, transport):
        self.transport = transport

    def summarize(self, items):
        return self.transport.post("/summaries", {"items": items})
