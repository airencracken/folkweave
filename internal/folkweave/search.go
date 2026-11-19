package folkweave

import (
	"context"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"modernc.org/sqlite"
)

const searchLength = 120

// SQLite's built-in lower only recognizes ASCII. Fold case in Go so the
// result excerpt and the database match letters the same way.
func init() {
	sqlite.MustRegisterDeterministicScalarFunction("folkweave_lower", 1,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			text, _ := args[0].(string)
			return strings.ToLower(text), nil
		})
}

const (
	postMatch = iota
	replyMatch
	photoMatch
)

type searchPosition [5]int64 // timestamp, post ID, kind, reply ID, photo position

type searchHit struct {
	position searchPosition
	Author   string
	Text     string
	PhotoID  string
}

func (h searchHit) Kind() string {
	switch h.position[2] {
	case replyMatch:
		return "Reply"
	case photoMatch:
		return "Photo description"
	default:
		return "Post"
	}
}

func (h searchHit) URL() string {
	switch h.position[2] {
	case replyMatch:
		return fmt.Sprintf("/posts/%d#reply-%d", h.position[1], h.position[3])
	case photoMatch:
		return photoViewURL(h.position[1], h.PhotoID)
	default:
		return (Post{ID: h.position[1]}).URL()
	}
}

type searchPage struct {
	Query string
	Hits  []searchHit
	Next  string
}

func searchBefore(raw string) (searchPosition, error) {
	if len(raw) > 120 {
		return searchPosition{}, ErrInvalid
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	var parts []int64
	if err != nil || json.Unmarshal(data, &parts) != nil || len(parts) != 5 || parts[1] < 1 ||
		parts[2] < postMatch || parts[2] > photoMatch || parts[3] < 0 || parts[4] < 0 {
		return searchPosition{}, ErrInvalid
	}
	return searchPosition(parts), nil
}

func (s *Store) search(ctx context.Context, q, before string) (searchPage, error) {
	q = strings.TrimSpace(q)
	page := searchPage{Query: q}
	if !validText(q, searchLength, false) {
		return page, ErrInvalid
	}
	if q == "" && before == "" {
		return page, nil
	}
	var cursor searchPosition
	if before != "" {
		var err error
		cursor, err = searchBefore(before)
		if err != nil || q == "" {
			return page, ErrInvalid
		}
	}
	needle := strings.ToLower(q)
	rows, err := s.db.QueryContext(ctx, `
WITH hits(stamp,post_id,kind,reply_id,position,photo_id,author,body) AS (
 SELECT p.created_at,p.id,0,0,0,'',u.name,p.body
 FROM posts p JOIN users u ON p.user_id=u.id
 WHERE p.repost_kind IS NULL AND instr(folkweave_lower(p.body),?)
 UNION ALL
 SELECT r.created_at,r.post_id,1,r.id,0,'',u.name,r.body
 FROM replies r JOIN users u ON r.user_id=u.id
 WHERE instr(folkweave_lower(r.body),?)
 UNION ALL
 SELECT p.created_at,p.id,2,0,f.position,f.id,u.name,f.alt
 FROM photos f JOIN posts p ON f.post_id=p.id JOIN users u ON p.user_id=u.id
 WHERE instr(folkweave_lower(f.alt),?)
)
SELECT stamp,post_id,kind,reply_id,position,photo_id,author,body FROM hits
WHERE (?='' OR (stamp,post_id,kind,reply_id,position) < (?,?,?,?,?))
ORDER BY stamp DESC,post_id DESC,kind DESC,reply_id DESC,position DESC LIMIT ?`,
		needle, needle, needle, before, cursor[0], cursor[1], cursor[2], cursor[3], cursor[4], PageSize+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var hit searchHit
		pos := &hit.position
		if err := rows.Scan(&pos[0], &pos[1], &pos[2], &pos[3], &pos[4], &hit.PhotoID, &hit.Author, &hit.Text); err != nil {
			return page, err
		}
		if len(page.Hits) == PageSize {
			data, _ := json.Marshal(page.Hits[PageSize-1].position)
			params := url.Values{"q": {q}, "before": {base64.RawURLEncoding.EncodeToString(data)}}
			page.Next = "/search?" + params.Encode()
			break
		}
		hit.Text = searchExcerpt(hit.Text, needle)
		page.Hits = append(page.Hits, hit)
	}
	return page, rows.Err()
}

func searchExcerpt(text, needle string) string {
	folded := strings.ToLower(text)
	offset := strings.Index(folded, needle)
	if offset < 0 {
		return text
	}
	runes := []rune(text)
	match := utf8.RuneCountInString(folded[:offset])
	start := max(0, match-65)
	end := min(len(runes), max(start+200, match+utf8.RuneCountInString(needle)))
	excerpt := strings.TrimSpace(string(runes[start:end]))
	if start > 0 {
		excerpt = "…" + excerpt
	}
	if end < len(runes) {
		excerpt += "…"
	}
	return excerpt
}

func (a *App) search(w http.ResponseWriter, r *http.Request) {
	const invalid = "Enter a search up to 120 characters."
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(r.URL.RawQuery) > 2048 || len(values["q"]) > 1 || len(values["before"]) > 1 {
		a.render(w, r, http.StatusBadRequest, Page{View: "search", Title: "Search", Error: invalid})
		return
	}
	result, err := a.store.search(r.Context(), values.Get("q"), values.Get("before"))
	if err == ErrInvalid {
		a.render(w, r, http.StatusBadRequest, Page{View: "search", Title: "Search", Error: invalid})
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, Page{View: "search", Title: "Search", Search: result})
}
