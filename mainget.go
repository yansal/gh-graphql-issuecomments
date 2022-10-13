package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

func main() {
	ctx := context.Background()
	httpclient := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")}))
	client := githubv4.NewClient(httpclient)

	var q Q
	variables := map[string]interface{}{
		"issuescursor":                       (*githubv4.String)(nil),
		"issuecommentscursor":                (*githubv4.String)(nil),
		"pullrequestscursor":                 (*githubv4.String)(nil),
		"repositorydiscussionscursor":        (*githubv4.String)(nil),
		"repositorydiscussioncommentscursor": (*githubv4.String)(nil),
		"first":                              githubv4.Int(100),
		"login":                              githubv4.String("rsc"),
	}
	if err := client.Query(ctx, &q, variables); err != nil {
		log.Fatal(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(q); err != nil {
		log.Fatal(err)
	}
}
