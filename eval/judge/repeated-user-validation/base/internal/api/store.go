package api

import "io"

type userStore interface {
    Delete(string)
    Update(string, io.Reader)
}

var store userStore
