package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/softwarity/meerkat/internal/store"
)

// The files uploaded on a route (ROUTE-22): what a route in the "files" mode
// serves under its own path. Routing plane, like the route itself: root or
// infra-admin.
func (a *API) registerRouteFiles(mux Mux) {
	mux.Handle("GET /api/routes/{id}/files", a.gw(a.listRouteFiles))
	mux.Handle("PUT /api/routes/{id}/files/{name...}", a.infraAdmin(a.putRouteFile))
	mux.Handle("PATCH /api/routes/{id}/files/{name...}", a.infraAdmin(a.updateRouteFile))
	mux.Handle("DELETE /api/routes/{id}/files/{name...}", a.infraAdmin(a.deleteRouteFile))
}

func (a *API) listRouteFiles(w http.ResponseWriter, r *http.Request) {
	route, err := a.st.GetRoute(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "route not found")
		return
	}
	files, err := a.st.ListRouteFiles(r.Context(), route.ID)
	if err != nil {
		a.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, files)
}

// putRouteFile stores the body as the file of that name, replacing one that
// had it. The body is the file itself; its type is read from its name.
func (a *API) putRouteFile(w http.ResponseWriter, r *http.Request, actor store.User) {
	route, err := a.st.GetRoute(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "route not found")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, store.MaxRouteFileBytes+1))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "could not read the file: "+err.Error())
		return
	}
	if len(body) > store.MaxRouteFileBytes {
		writeErr(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("this file is over %d bytes: a file that big does not belong in a configuration package", store.MaxRouteFileBytes))
		return
	}
	f, err := a.st.SetRouteFile(r.Context(), route.ID, r.PathValue("name"), body)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := a.reloadRouting(r.Context()); err != nil {
		a.internal(w, fmt.Errorf("saved, but reload failed: %w", err))
		return
	}
	a.auditEvent(r.Context(), actor, "route.file", "route", route.ID, route.Name, "",
		fmt.Sprintf("%s (%d bytes, %s)", f.Name, f.Size, f.ContentType))
	f.Data = nil
	writeJSON(w, http.StatusOK, f)
}

// updateRouteFile renames a file or corrects its type: {"name", "contentType"},
// either one, an absent one kept.
func (a *API) updateRouteFile(w http.ResponseWriter, r *http.Request, actor store.User) {
	route, err := a.st.GetRoute(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "route not found")
		return
	}
	var in struct {
		Name        string `json:"name"`
		ContentType string `json:"contentType"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "a JSON object with name and/or contentType: "+err.Error())
		return
	}
	old := r.PathValue("name")
	f, err := a.st.UpdateRouteFile(r.Context(), route.ID, old, in.Name, in.ContentType)
	switch {
	case errors.Is(err, store.ErrNoRouteFile):
		writeErr(w, http.StatusNotFound, err.Error())
		return
	case err != nil:
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := a.reloadRouting(r.Context()); err != nil {
		a.internal(w, fmt.Errorf("saved, but reload failed: %w", err))
		return
	}
	a.auditEvent(r.Context(), actor, "route.file.update", "route", route.ID, route.Name, "",
		fmt.Sprintf("%s -> %s (%s)", old, f.Name, f.ContentType))
	writeJSON(w, http.StatusOK, f)
}

func (a *API) deleteRouteFile(w http.ResponseWriter, r *http.Request, actor store.User) {
	route, err := a.st.GetRoute(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "route not found")
		return
	}
	name := r.PathValue("name")
	gone, err := a.st.DeleteRouteFile(r.Context(), route.ID, name)
	if err != nil {
		a.internal(w, err)
		return
	}
	if !gone {
		writeErr(w, http.StatusNotFound, errors.New("no file "+name+" on this route").Error())
		return
	}
	if err := a.reloadRouting(r.Context()); err != nil {
		a.internal(w, fmt.Errorf("removed, but reload failed: %w", err))
		return
	}
	a.auditEvent(r.Context(), actor, "route.file.delete", "route", route.ID, route.Name, "", name)
	w.WriteHeader(http.StatusNoContent)
}
