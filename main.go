package main

import (
	"bytes"
	"context"
	_ "embed"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"

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
	return http.ListenAndServe(":8080", s)
}

func newserver() (*server, error) {
	ctx := context.Background()
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")})
	httpclient := oauth2.NewClient(ctx, src)

	t, err := template.New("name").Parse(templatestr)
	if err != nil {
		return nil, err
	}

	return &server{
		client:   githubv4.NewClient(httpclient),
		template: t,
	}, nil
}

type server struct {
	client   *githubv4.Client
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
		ctx := r.Context()
		if err := s.client.Query(ctx, &q, map[string]interface{}{
			"login": githubv4.String(query.Get("login")),
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := s.template.Execute(b, q); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	io.Copy(w, b)
}
