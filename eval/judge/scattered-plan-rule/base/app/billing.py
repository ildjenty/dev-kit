from .plans import base_lines, priority_support_line


def invoice_lines(user):
    lines = base_lines(user)
    return lines
