from .documents import read_document


def download(session, document_id, store):
    return read_document(session.actor, document_id, store)
