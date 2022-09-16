package main

import (
	"bytes"
	_ "embed"
	"fmt"
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

func main1() error {
	s, err := newserver()
	if err != nil {
		return err
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return http.ListenAndServe(":"+port, s)
}

func newserver() (*server, error) {
	t, err := template.New("name").Parse(templatestr)
	if err != nil {
		return nil, err
	}

	return &server{
		src:      oauth2.StaticTokenSource(&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")}),
		template: t,
	}, nil
}

type server struct {
	src      oauth2.TokenSource
	template *template.Template
}

//go:embed template.html
var templatestr string

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b := new(bytes.Buffer)
	var q struct {
		User struct {
			IssueComments struct {
				Nodes []struct {
					CreatedAt githubv4.String
					UpdatedAt githubv4.String
					Issue     struct {
						Title githubv4.String
					}
					Repository struct {
						NameWithOwner githubv4.String
					}
					URL githubv4.String
				}
			} `graphql:"issueComments(first:100, orderBy:{direction:DESC, field:UPDATED_AT})"`
		} `graphql:"user(login: $login)"`
	}

	if query := r.URL.Query(); query.Has("login") {
		h := w.Header()
		h.Set("content-type", "text/html; charset=utf-8")

		ctx := r.Context()
		httpclient := oauth2.NewClient(ctx, s.src)
		httpclient.Transport = newwrappedroundtripper(httpclient.Transport, w)
		client := githubv4.NewClient(httpclient)

		if err := client.Query(ctx, &q, map[string]interface{}{
			"login": githubv4.String(query.Get("login")),
		}); err != nil {
			log.Print(err)
			return
		}
		fmt.Fprint(w, `<script>document.body.innerHTML="" </script>`)
	}

	if err := s.template.Execute(b, q); err != nil {
		log.Print(err)
		return
	}
	io.Copy(w, b)
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
			fmt.Fprintf(w, "<pre>%s</pre>", dump)
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
			fmt.Fprintf(w, "<pre>%s</pre>", dump)
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
