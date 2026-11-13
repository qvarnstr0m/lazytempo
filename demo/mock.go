//go:build ignore

// Command mock is a throwaway mock of the Jira + Tempo APIs used to record the
// lazytempo demo (see demo/README.md). It serves fictional data only — no real
// Jira/Tempo account, no real customers.
//
// It listens on :8787 and routes both Jira (/rest/api/3/...) and Tempo
// (/accounts, /worklogs, ...) paths. The demo points the app at a vanity host
// (http://hollywood-upstairs.atlassian.net) and reaches this server via
// HTTP_PROXY, so the
// recording shows a realistic Jira site instead of localhost.
//
// It is excluded from normal builds (`//go:build ignore`), so run it by naming
// the file explicitly:
//
//	go run demo/mock.go        # or: go build -o mock demo/mock.go && ./mock
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const addr = ":8787"

// ---- fictional reference data ----

type issue struct {
	id, key, summary, projID, projKey, status string
	estimate                                  int
}

var issues = []issue{
	{"10001", "SURG-101", "Homer's coronary bypass", "100", "SURG", "In Progress", 14400},
	{"10002", "SURG-102", "Reattach thumb (either hand)", "100", "SURG", "To Do", 28800},
	{"10003", "SURG-103", "Locate the appendix (see diagram)", "100", "SURG", "In Review", 7200},
	{"10004", "HAMC-7", "Avoid the coroner", "200", "HAMC", "In Progress", 10800},
	{"10005", "HAMC-12", "Renew medical license (again)", "200", "HAMC", "To Do", 0},
	{"10006", "CLINIC-5", "Restock gauze and chainsaw oil", "300", "CLINIC", "To Do", 0},
	{"10007", "SURG-104", "Research the inverse appendectomy", "100", "SURG", "In Progress", 0},
}

type account struct {
	key, name, status string
	id                int
}

var accounts = []account{
	{"SURGERY", "Discount Surgery", "OPEN", 1},
	{"SUPPLY", "Gauze & Supplies", "OPEN", 2},
	{"LEGAL", "Malpractice Defense", "OPEN", 3},
	{"OLD", "Expired License 2023", "CLOSED", 4},
}

// accountLink ties an account id to a project, flagging the project default.
type accountLink struct {
	id    int
	deflt bool
}

// project id -> linked accounts (first linked default shown first)
var links = map[string][]accountLink{
	"100": {{1, true}, {2, false}}, // SURG: Discount Surgery default
	"200": {{3, true}, {1, false}}, // HAMC: Malpractice Defense default
	"300": {{2, true}},             // CLINIC: Gauze & Supplies default
}

// ---- worklog store (seeded + created during the demo) ----

type kv struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type wl struct {
	ID      int    `json:"tempoWorklogId"`
	Seconds int    `json:"timeSpentSeconds"`
	Date    string `json:"startDate"`
	Time    string `json:"startTime"`
	Desc    string `json:"description"`
	Issue   struct {
		ID int `json:"id"`
	} `json:"issue"`
	Author struct {
		AccountID string `json:"accountId"`
	} `json:"author"`
	Attributes struct {
		Values []kv `json:"values"`
	} `json:"attributes"`
}

func newWL(id, issueID, seconds int, date, t, desc, acct string) wl {
	var w wl
	w.ID = id
	w.Seconds = seconds
	w.Date = date
	w.Time = t
	w.Desc = desc
	w.Issue.ID = issueID
	w.Author.AccountID = "demo-user"
	w.Attributes.Values = []kv{}
	if acct != "" {
		w.Attributes.Values = []kv{{Key: "_Account_", Value: acct}}
	}
	return w
}

var (
	mu     sync.Mutex
	store  []wl
	nextID = 9100
)

// mondayOf returns the Monday of the given day's week (matching the app).
func mondayOf(now time.Time) time.Time {
	d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	off := (int(d.Weekday()) + 6) % 7
	return d.AddDate(0, 0, -off)
}

// seed fills the current week Mon–Fri with plausible worklogs, so the demo
// always shows a populated week regardless of when it is recorded.
func seed() {
	mon := mondayOf(time.Now())
	day := func(n int) string { return mon.AddDate(0, 0, n).Format("2006-01-02") }
	store = []wl{
		newWL(9001, 10001, 2*3600, day(0), "09:00:00", "Operate on Homer (read manual first)", "SURGERY"),
		newWL(9002, 10003, 1*3600, day(0), "11:00:00", "Hunt for the appendix", "SURGERY"),
		newWL(9003, 10004, 3*3600, day(0), "13:00:00", "Keep clear of the coroner", "LEGAL"),
		newWL(9004, 10002, 4*3600, day(1), "09:00:00", "Reattach thumb (probably the right one)", "SURGERY"),
		newWL(9005, 10006, 2*3600, day(1), "14:00:00", "Reorder gauze in bulk", "SUPPLY"),
		newWL(9006, 10002, 1*3600, day(2), "09:00:00", "Quick checkup", ""), // no account
		newWL(9007, 10001, 5*3600, day(2), "10:00:00", "Bypass surgery, take two", "SURGERY"),
		newWL(9008, 10005, 3*3600, day(3), "09:00:00", "License renewal paperwork", "LEGAL"),
		newWL(9009, 10006, 2*3600, day(4), "09:00:00", "Inventory: hedge clippers", "SUPPLY"),
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func issueAttrs(acct []kv) string {
	for _, a := range acct {
		if a.Key == "_Account_" {
			return a.Value
		}
	}
	return ""
}

func main() {
	seed()
	mux := http.NewServeMux()

	// --- Jira ---
	mux.HandleFunc("/rest/api/3/myself", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"accountId": "demo-user", "displayName": "Dr. Nick Riviera"})
	})

	mux.HandleFunc("/rest/api/3/search/jql", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			JQL string `json:"jql"`
		}
		_ = json.Unmarshal(body, &req)

		sel := issues
		if i := strings.Index(req.JQL, "id IN ("); i >= 0 {
			rest := req.JQL[i+len("id IN ("):]
			if end := strings.IndexByte(rest, ')'); end >= 0 {
				wanted := map[string]bool{}
				for _, part := range strings.Split(rest[:end], ",") {
					wanted[strings.TrimSpace(part)] = true
				}
				sel = nil
				for _, is := range issues {
					if wanted[is.id] {
						sel = append(sel, is)
					}
				}
			}
		}

		type jiraIssue struct {
			ID     string `json:"id"`
			Key    string `json:"key"`
			Fields struct {
				Summary string `json:"summary"`
				Status  struct {
					Name string `json:"name"`
				} `json:"status"`
				Project struct {
					ID  string `json:"id"`
					Key string `json:"key"`
				} `json:"project"`
				TimeEstimate int `json:"timeestimate"`
			} `json:"fields"`
		}
		out := struct {
			Issues        []jiraIssue `json:"issues"`
			NextPageToken string      `json:"nextPageToken"`
			IsLast        bool        `json:"isLast"`
		}{IsLast: true}
		for _, is := range sel {
			var ji jiraIssue
			ji.ID = is.id
			ji.Key = is.key
			ji.Fields.Summary = is.summary
			ji.Fields.Status.Name = is.status
			ji.Fields.Project.ID = is.projID
			ji.Fields.Project.Key = is.projKey
			ji.Fields.TimeEstimate = is.estimate
			out.Issues = append(out.Issues, ji)
		}
		writeJSON(w, out)
	})

	// --- Tempo ---
	mux.HandleFunc("/work-attributes", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"results": []map[string]string{
				{"key": "_Account_", "name": "Account", "type": "ACCOUNT"},
			},
		})
	})

	mux.HandleFunc("/accounts", func(w http.ResponseWriter, r *http.Request) {
		type a struct {
			Key    string `json:"key"`
			ID     int    `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		}
		var res []a
		for _, ac := range accounts {
			res = append(res, a{ac.key, ac.id, ac.name, ac.status})
		}
		writeJSON(w, map[string]any{"results": res, "metadata": map[string]string{"next": ""}})
	})

	mux.HandleFunc("/account-links/project/", func(w http.ResponseWriter, r *http.Request) {
		proj := strings.TrimPrefix(r.URL.Path, "/account-links/project/")
		type res struct {
			Account struct {
				ID   int    `json:"id"`
				Self string `json:"self"`
			} `json:"account"`
			Scope struct {
				ID int `json:"id"`
			} `json:"scope"`
			Default bool `json:"default"`
		}
		var out []res
		for _, l := range links[proj] {
			var rr res
			rr.Account.ID = l.id
			rr.Account.Self = fmt.Sprintf("http://hollywood-upstairs.atlassian.net/accounts/%d", l.id)
			pid, _ := strconv.Atoi(proj)
			rr.Scope.ID = pid
			rr.Default = l.deflt
			out = append(out, rr)
		}
		writeJSON(w, map[string]any{"results": out})
	})

	// List the user's worklogs.
	mux.HandleFunc("/worklogs/user/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		writeJSON(w, map[string]any{"results": store, "metadata": map[string]string{"next": ""}})
	})

	// Create a worklog.
	mux.HandleFunc("/worklogs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var p struct {
			IssueID int    `json:"issueId"`
			Seconds int    `json:"timeSpentSeconds"`
			Date    string `json:"startDate"`
			Time    string `json:"startTime"`
			Desc    string `json:"description"`
			Attrs   []kv   `json:"attributes"`
		}
		_ = json.Unmarshal(body, &p)
		mu.Lock()
		id := nextID
		nextID++
		nw := newWL(id, p.IssueID, p.Seconds, p.Date, p.Time, p.Desc, issueAttrs(p.Attrs))
		store = append(store, nw)
		mu.Unlock()
		writeJSON(w, nw)
	})

	// Update or delete a worklog by id.
	mux.HandleFunc("/worklogs/", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/worklogs/"))
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodDelete:
			for i, x := range store {
				if x.ID == id {
					store = append(store[:i], store[i+1:]...)
					break
				}
			}
			w.WriteHeader(http.StatusNoContent)
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			var p struct {
				IssueID int    `json:"issueId"`
				Seconds int    `json:"timeSpentSeconds"`
				Date    string `json:"startDate"`
				Time    string `json:"startTime"`
				Desc    string `json:"description"`
				Attrs   []kv   `json:"attributes"`
			}
			_ = json.Unmarshal(body, &p)
			upd := newWL(id, p.IssueID, p.Seconds, p.Date, p.Time, p.Desc, issueAttrs(p.Attrs))
			for i, x := range store {
				if x.ID == id {
					store[i] = upd
					break
				}
			}
			writeJSON(w, upd)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	log.Printf("mock Jira+Tempo API listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, logReq(mux)))
}

func logReq(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		h.ServeHTTP(w, r)
	})
}
