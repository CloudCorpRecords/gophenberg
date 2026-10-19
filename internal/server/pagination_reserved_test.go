// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/server"
)

// errorMeta reads the data a refused request carries.
func errorMeta(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var answered struct {
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &answered); err != nil {
		t.Fatalf("reading the error response: %v", err)
	}
	return answered.Meta
}

func TestTypeAPIKeepsPageReservedForPagination(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	for _, request := range []struct {
		method string
		path   string
		body   string
	}{
		{
			method: http.MethodPost, path: "/api/types",
			body: `{"key":"chapter","singular_label":"Chapter","plural_label":"Chapters","route_word":"page"}`,
		},
		{method: http.MethodPatch, path: "/api/types/post", body: `{"route_word":"page"}`},
	} {
		recorder := doRequest(t, handler, request.method, request.path, request.body)
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s %s status = %d: %s", request.method, request.path, recorder.Code, recorder.Body.String())
		}
		if code := errorCode(t, recorder); code != "route_word_reserved" {
			t.Errorf("code = %q, want route_word_reserved", code)
		}
	}
}

func TestTypeAPICanEditALegacyPageTypeWithoutMovingIt(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	types := newFakeTypeStore()
	legacy := pageType()
	legacy.RouteWord = "page"
	types.register(legacy)
	handler := authedServerWithStores(t, server.Config{Users: users, Content: newFakePostStore(), Types: types})
	recorder := doRequest(t, handler, http.MethodPatch, "/api/types/page", `{"description":"Site pages"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("PATCH status = %d: %s", recorder.Code, recorder.Body.String())
	}
	if body := decodeBody[typeBody](t, recorder); body.RouteWord != "page" || body.Description != "Site pages" {
		t.Errorf("body = %+v, want the edited type at its existing route", body)
	}
}

func TestContentAPIRefusesAChildAtTheRootPaginationAddress(t *testing.T) {
	t.Parallel()

	handler := authedTypeServer(t)
	changed := doRequest(t, handler, http.MethodPatch, "/api/types/post", `{"hierarchical":true}`)
	if changed.Code != http.StatusOK {
		t.Fatalf("PATCH type status = %d: %s", changed.Code, changed.Body.String())
	}
	created := doRequest(t, handler, http.MethodPost, "/api/content", `{"title":"Page"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("POST parent status = %d: %s", created.Code, created.Body.String())
	}
	parent := decodeBody[postBody](t, created)
	recorder := doRequest(t, handler, http.MethodPost, "/api/content",
		`{"title":"2","parent_id":"`+parent.ID.String()+`"}`)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST child status = %d: %s", recorder.Code, recorder.Body.String())
	}
	if code := errorCode(t, recorder); code != "address_reserved" {
		t.Errorf("code = %q, want address_reserved", code)
	}
}

func TestTypeAPINamesTheLabelBlockingARootHandOver(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	types := newFakeTypeStore()
	types.register(pageType())
	types.updateErr = content.LabelRouteWordReserved(content.Type{PluralLabel: "Page", RouteWord: "page"})
	handler := authedServerWithStores(t, server.Config{Users: users, Content: newFakePostStore(), Types: types})

	recorder := doRequest(t, handler, http.MethodPatch, "/api/types/page", `{"description":"Site pages"}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("PATCH status = %d: %s", recorder.Code, recorder.Body.String())
	}
	if code := errorCode(t, recorder); code != "route_word_reserved_by_label" {
		t.Errorf("code = %q, want route_word_reserved_by_label", code)
	}
	if meta := errorMeta(t, recorder); meta["label"] != "Page" || meta["word"] != "page" {
		t.Errorf("meta = %v, want the label Page and the word page named", meta)
	}
}

func TestTypeAPINamesTheAddressBlockingARootHandOver(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	types := newFakeTypeStore()
	types.register(pageType())
	types.updateErr = content.PaginationAddressReserved("page/2")
	handler := authedServerWithStores(t, server.Config{Users: users, Content: newFakePostStore(), Types: types})

	recorder := doRequest(t, handler, http.MethodPatch, "/api/types/page", `{"description":"Site pages"}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("PATCH status = %d: %s", recorder.Code, recorder.Body.String())
	}
	if code := errorCode(t, recorder); code != "pagination_address_reserved" {
		t.Errorf("code = %q, want pagination_address_reserved", code)
	}
	if meta := errorMeta(t, recorder); meta["path"] != "page/2" {
		t.Errorf("meta = %v, want the path page/2 named", meta)
	}
}
