def base_lines(user):
    return [{"name": "subscription", "amount": user.monthly_price}]


def priority_support_line():
    return {"name": "priority support", "amount": 0}
