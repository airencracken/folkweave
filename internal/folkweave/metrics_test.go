package folkweave

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestOwnerMetricsTotalsPermissionsAndFailure(t *testing.T) {
	s, a, owner := webFixture(t)
	owner.signIn(t, "owner")
	testMember(t, s, "member")
	if _, err := s.CreatePost(t.Context(), 1, "private-metric-fixture", testKey(), nil); err != nil {
		t.Fatal(err)
	}
	metrics, err := s.Metrics(t.Context(), time.Now())
	if err != nil || len(metrics) < 6 || metrics[0].Value != 2 || metrics[3].Value != 1 {
		t.Fatal(metrics, err)
	}
	for _, path := range []string{"/admin/metrics", "/admin/metrics?role=owner&owner=true"} {
		w := owner.request("GET", path, nil, nil)
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Body.String(), "Instance overview") || strings.Contains(w.Body.String(), "private-metric-fixture") {
			t.Fatal("owner metrics", w.Code, w.Body.String())
		}
	}
	member := &testClient{app: a, cookies: map[string]*http.Cookie{}}
	member.signIn(t, "member")
	guest := &testClient{app: a, cookies: map[string]*http.Cookie{}}
	for _, client := range []*testClient{member, guest} {
		for _, path := range []string{"/admin/metrics", "/admin/metrics?role=owner"} {
			w := client.request("GET", path, nil, nil)
			if w.Code != 303 && w.Code != 403 {
				t.Fatal("metrics authorization", w.Code)
			}
			if strings.Contains(w.Body.String(), "Instance overview") {
				t.Fatal("metrics leaked")
			}
		}
	}
	if _, err = s.db.Exec("ALTER TABLE posts RENAME TO unavailable_posts"); err != nil {
		t.Fatal(err)
	}
	if w := owner.request("GET", "/admin/metrics", nil, nil); w.Code != 500 || strings.Contains(w.Body.String(), "Instance overview") {
		t.Fatal("partial metrics published", w.Code)
	}
}
