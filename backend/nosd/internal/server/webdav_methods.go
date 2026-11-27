package server

import "github.com/go-chi/chi/v5"

// chi answers 405 to methods it doesn't know, even for a mounted handler,
// so WebDAV's own methods must be registered before any router is built.
// (They weren't: PROPFIND, MKCOL, MOVE, ... all got 405, so file managers
// couldn't list a share and sync clients couldn't make folders.)
func init() {
	for _, m := range []string{"PROPFIND", "PROPPATCH", "MKCOL", "COPY", "MOVE", "LOCK", "UNLOCK"} {
		chi.RegisterMethod(m)
	}
}
