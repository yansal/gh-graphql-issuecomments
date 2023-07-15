package main

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ardanlabs/conf/v3"
	"github.com/google/go-github/v53/github"
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

	var h handler
	mux.HandleFunc("/", h.root)
	mux.HandleFunc("/search", h.search)
	mux.HandleFunc("/query", h.query)

	return http.ListenAndServe(":"+cfg.Port, mux)
}

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
		Name:  cookiename,
		Value: token.AccessToken,
	})
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (o *oauth2handler) login(w http.ResponseWriter, r *http.Request) {
	authURL := o.cfg.AuthCodeURL(o.state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func maketemplates() *templates {
	index := template.Must(template.
		New("index.html").
		ParseFS(os.DirFS("."),
			"templates/index.html",
		),
	)
	query := template.Must(template.
		New("query.html").
		Funcs(template.FuncMap{
			"ago": func(value githubv4.String) (string, error) {
				t, err := time.Parse(time.RFC3339, string(value))
				if err != nil {
					return "", err
				}
				return timeago.English.Format(t), nil
			},
		}).
		ParseFS(os.DirFS("."),
			"templates/query.html",
			"templates/partials/*.html",
		),
	)
	search := template.Must(template.
		New("search.html").
		ParseFS(os.DirFS("."),
			"templates/search.html",
		),
	)

	return &templates{
		index:  index,
		query:  query,
		search: search,
	}
}

type templates struct {
	// full
	index *template.Template

	// fragments
	query  *template.Template
	search *template.Template
}
type handler struct{}

func (h *handler) root(w http.ResponseWriter, r *http.Request) {
	var (
		authenticated bool
		ctx           = r.Context()
		httpclient    *http.Client
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
			authenticated = true
		}
	}

	b := new(bytes.Buffer)
	if err := maketemplates().index.Execute(b, authenticated); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	io.Copy(w, b)
}

func (h *handler) search(w http.ResponseWriter, r *http.Request) {
	var (
		ctx         = r.Context()
		cookie, err = r.Cookie(cookiename)
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	login := r.FormValue("login")
	if login == "" {
		return
	}

	var (
		src        = oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cookie.Value})
		httpclient = oauth2.NewClient(ctx, src)
		client     = github.NewClient(httpclient)
	)
	res, _, err := client.Search.Users(ctx, login, &github.SearchOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	b := new(bytes.Buffer)
	if err := maketemplates().search.Execute(b, res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	io.Copy(w, b)
}
func (h *handler) query(w http.ResponseWriter, r *http.Request) {
	var (
		ctx         = r.Context()
		cookie, err = r.Cookie(cookiename)
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	var (
		src        = oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cookie.Value})
		httpclient = oauth2.NewClient(ctx, src)
		client     = githubv4.NewClient(httpclient)
		query      = r.URL.Query()
		variables  = map[string]interface{}{
			"cursor": (*githubv4.String)(nil),
			"login":  githubv4.String(query.Get("login")),
		}
	)
	if query.Has("cursor") {
		variables["cursor"] = githubv4.String(query.Get("cursor"))
	}
	q := new(githubquery)
	if err := client.Query(ctx, q, variables); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	b := new(bytes.Buffer)
	if err := maketemplates().query.Execute(b, q); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	io.Copy(w, b)
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
