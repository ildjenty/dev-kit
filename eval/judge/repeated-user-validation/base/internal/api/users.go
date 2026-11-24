package api

import (
    "net/http"
    "strings"
)

func validUserID(id string) bool {
    return len(id) == 26 && strings.HasPrefix(id, "usr_")
}

func DeleteUser(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    if !validUserID(id) {
        http.Error(w, "bad id", 400)
        return
    }
    store.Delete(id)
}

func UpdateUser(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    store.Update(id, r.Body)
    w.WriteHeader(204)
}
