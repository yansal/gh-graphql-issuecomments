package main

import (
	"bytes"
	"embed"
	"fmt"
	"html"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"time"

	"github.com/henvic/httpretty"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

func main() {
	log.SetFlags(0)

	http.DefaultTransport = (&httpretty.Logger{
		RequestHeader:  true,
		RequestBody:    true,
		ResponseHeader: true,
		ResponseBody:   true,
	}).RoundTripper(http.DefaultTransport)

	if err := main1(); err != nil {
		log.Fatal(err)
	}
}

var (
	//go:embed static
	staticfs embed.FS
	//go:embed templates
	tmplfs embed.FS
)

func main1() error {
	h, err := newhandler()
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServer(http.FS(staticfs)))
	mux.Handle("/favicon.ico", http.NotFoundHandler())
	mux.Handle("/", h)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return http.ListenAndServe(":"+port, mux)
}

func newhandler() (http.Handler, error) {
	tmpl, err := template.ParseFS(tmplfs, "templates/*")
	if err != nil {
		return nil, err
	}
	return &handler{
		src:  oauth2.StaticTokenSource(&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")}),
		tmpl: tmpl,
	}, nil
}

type handler struct {
	src  oauth2.TokenSource
	tmpl *template.Template
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b := new(bytes.Buffer)
	if err := h.tmpl.ExecuteTemplate(b, "first", nil); err != nil {
		fmt.Fprint(w, err)
		return
	}
	if _, err := io.Copy(w, b); err != nil {
		return
	}

	var q struct {
		User *struct {
			Login         githubv4.String
			IssueComments struct {
				Nodes []struct {
					BodyText  githubv4.String
					CreatedAt githubv4.String
					UpdatedAt githubv4.String
					Issue     struct {
						Title githubv4.String
					}
					ReactionGroups []struct {
						Content  githubv4.String
						Reactors struct {
							TotalCount githubv4.Int
						}
					}
					Repository struct {
						NameWithOwner githubv4.String
					}
					URL githubv4.String
				}
				PageInfo struct {
					EndCursor   githubv4.String
					HasNextPage bool
				}
			} `graphql:"issueComments(first: 100, after: $cursor, orderBy:{direction:DESC, field:UPDATED_AT})"`
		} `graphql:"user(login: $login)"`
	}

	if query := r.URL.Query(); query.Has("login") {
		ctx := r.Context()
		httpclient := oauth2.NewClient(ctx, h.src)
		httpclient.Transport = newwrappedroundtripper(httpclient.Transport, w)
		client := githubv4.NewClient(httpclient)

		variables := map[string]interface{}{
			"cursor": (*githubv4.String)(nil),
			"login":  githubv4.String(query.Get("login")),
		}
		if query.Has("cursor") {
			variables["cursor"] = githubv4.String(query.Get("cursor"))
		}
		if err := client.Query(ctx, &q, variables); err != nil {
			log.Print(err)
			return
		}
		fmt.Fprint(w, `<script>document.body.innerHTML=""</script>`)
	}

	b.Reset()
	if err := h.tmpl.ExecuteTemplate(b, "second", q); err != nil {
		fmt.Fprint(w, err)
		return
	}
	if _, err := io.Copy(w, b); err != nil {
		return
	}
}

func newwrappedroundtripper(rt http.RoundTripper, w http.ResponseWriter) http.RoundTripper {
	done := make(chan struct{})
	return &wrappedroundtripper{
		before: func(req *http.Request) {
			body := true
			dump, err := httputil.DumpRequestOut(req, body)
			if err != nil {
				return
			}
			fmt.Fprintf(w, "<pre>%s</pre>", html.EscapeString(string(dump)))
			flusher := w.(http.Flusher)
			flusher.Flush()
			go func() {
				ticker := time.NewTicker(100 * time.Millisecond)
				defer ticker.Stop()
				for range ticker.C {
					select {
					case <-done:
						return
					default:
						fmt.Fprintf(w, "💩")
						flusher.Flush()
					}
				}
			}()
		},
		after: func(resp *http.Response, err error) {
			close(done)
			if err != nil {
				return
			}
			body := true
			dump, err := httputil.DumpResponse(resp, body)
			if err != nil {
				return
			}
			fmt.Fprintf(w, "<pre>%s</pre>", html.EscapeString(string(dump)))
			w.(http.Flusher).Flush()
		},
		wrapped: rt,
	}
}

type wrappedroundtripper struct {
	before  func(*http.Request)
	after   func(*http.Response, error)
	wrapped http.RoundTripper
}

func (rt *wrappedroundtripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.before != nil {
		rt.before(req)
	}
	resp, err := rt.wrapped.RoundTrip(req)
	if rt.after != nil {
		rt.after(resp, err)
	}
	return resp, err
}
