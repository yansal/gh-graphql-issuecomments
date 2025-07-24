package main

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/ardanlabs/conf/v3"
	"github.com/shurcooL/githubv4"
	"github.com/xeonx/timeago"
	"golang.org/x/oauth2"
)

func main() {
	log.SetFlags(0)
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

const cookiename = `github_access_token`

type config struct {
	GithubClientID     string `conf:"required"`
	GithubClientSecret string `conf:"required"`
	GithubRedirectURL  string `conf:"required"`
	GithubState        string `conf:"required"`
	Port               string `conf:"default:8080"`
}

func main1() error {
	var cfg config
	if help, err := conf.Parse("", &cfg); errors.Is(err, conf.ErrHelpWanted) {
		fmt.Println(help)
		return nil
	} else if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServer(http.FS(staticfs)))
	mux.HandleFunc("/favicon.ico", http.NotFound)

	o := &oauth2handler{
		cfg: &oauth2.Config{
			ClientID:     cfg.GithubClientID,
			ClientSecret: cfg.GithubClientSecret,
			RedirectURL:  cfg.GithubRedirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://github.com/login/oauth/authorize",
				TokenURL: "https://github.com/login/oauth/access_token",
			},
		},
		state: cfg.GithubState,
	}
	mux.HandleFunc("/oauth2_login", o.login)
	mux.HandleFunc("/oauth2_callback", o.callback)

	h, err := newhandler()
	if err != nil {
		return err
	}
	mux.Handle("/", logmw(h))

	slog.Info("listening on :" + cfg.Port)
	return http.ListenAndServe(":"+cfg.Port, mux)
}

func logmw(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &ResponseWriter{ResponseWriter: w}
		defer func() {
			slog.Info(r.Pattern,
				"duration", time.Since(start),
				"status", rw.statusCode,
			)
		}()
		h.ServeHTTP(rw, r)
	})
}

type ResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *ResponseWriter) Write(p []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(p)
}

func (rw *ResponseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.written = true
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *ResponseWriter) Flush() { rw.ResponseWriter.(http.Flusher).Flush() }

type oauth2handler struct {
	cfg   *oauth2.Config
	state string
}

func (o *oauth2handler) callback(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("state") != o.state {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	token, err := o.cfg.Exchange(r.Context(), r.FormValue("code"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:    cookiename,
		Value:   token.AccessToken,
		Expires: time.Now().AddDate(1, 0, 0), // 1 year
	})
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (o *oauth2handler) login(w http.ResponseWriter, r *http.Request) {
	authURL := o.cfg.AuthCodeURL(o.state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func newhandler() (http.Handler, error) {
	tmpl, err := template.
		New("template_name").
		Funcs(template.FuncMap{
			"ago": func(value githubv4.String) (string, error) {
				t, err := time.Parse(time.RFC3339, string(value))
				if err != nil {
					return "", err
				}
				return timeago.English.Format(t), nil
			},
		}).
		ParseFS(tmplfs, "templates/*")
	if err != nil {
		return nil, err
	}
	return &handler{
		tmpl: tmpl,
	}, nil
}

type handler struct {
	tmpl *template.Template
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		data       tmpldata
		ctx        = r.Context()
		httpclient *http.Client
	)
	if cookie, err := r.Cookie(cookiename); err == nil {
		// check if cookie is valid
		src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cookie.Value})
		httpclient = oauth2.NewClient(ctx, src)
		client := githubv4.NewClient(httpclient)
		var q struct {
			Viewer struct{ Login githubv4.String }
		}
		if err := client.Query(ctx, &q, nil); err == nil {
			data.Authenticated = true
		}
	}

	b := new(bytes.Buffer)
	if err := h.tmpl.ExecuteTemplate(b, "first", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(w, b); err != nil {
		return
	}

	if query := r.URL.Query(); data.Authenticated && query.Has("login") {
		httpclient.Transport = newwrappedroundtripper(httpclient.Transport, w)
		client := githubv4.NewClient(httpclient)

		variables := map[string]interface{}{
			"cursor": (*githubv4.String)(nil),
			"login":  githubv4.String(query.Get("login")),
		}
		if query.Has("cursor") {
			variables["cursor"] = githubv4.String(query.Get("cursor"))
		}
		data.Query = new(githubquery)
		if err := client.Query(ctx, data.Query, variables); err != nil {
			fmt.Fprintf(w, "<pre>%s</pre>", html.EscapeString(err.Error()))
			return
		}
		fmt.Fprint(w, `<script>document.body.innerHTML=""</script>`)
	}

	b.Reset()
	if err := h.tmpl.ExecuteTemplate(b, "second", data); err != nil {
		fmt.Fprint(w, err)
		return
	}
	if _, err := io.Copy(w, b); err != nil {
		return
	}
}

type tmpldata struct {
	Query         *githubquery
	Authenticated bool
}

type githubquery struct {
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

func newwrappedroundtripper(rt http.RoundTripper, w http.ResponseWriter) http.RoundTripper {
	flusher := w.(http.Flusher)
	done := make(chan struct{})
	return &wrappedroundtripper{
		before: func(req *http.Request) {
			body := true
			dump, err := httputil.DumpRequestOut(req, body)
			if err != nil {
				return
			}
			fmt.Fprintf(w, "<pre>%s</pre>", html.EscapeString(string(dump)))
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
			flusher.Flush()
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
