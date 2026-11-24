class PermissionDenied(Exception):
    pass


def read_document(actor, document_id, store):
    document = store.load(document_id)
    if document.owner_id != actor.id:
        raise PermissionDenied("document belongs to another user")
    return document.body
