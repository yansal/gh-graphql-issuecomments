package main

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"os"
	"sort"
	"time"
)

func main() {
	var q Q
	if err := json.NewDecoder(os.Stdin).Decode(&q); err != nil {
		log.Fatal(err)
	}
	var (
		items  []Item
		counts = struct {
			issues, issuecomments, pullrequests, repositorydiscussions, repositorydiscussioncomments int
		}{
			issues:                       len(q.User.Issues.Nodes),
			issuecomments:                len(q.User.IssueComments.Nodes),
			pullrequests:                 len(q.User.PullRequests.Nodes),
			repositorydiscussions:        len(q.User.RepositoryDiscussions.Nodes),
			repositorydiscussioncomments: len(q.User.RepositoryDiscussionComments.Nodes),
		}
	)
	for i := range q.User.Issues.Nodes {
		items = append(items, &q.User.Issues.Nodes[i])
	}
	for i := range q.User.IssueComments.Nodes {
		items = append(items, &q.User.IssueComments.Nodes[i])
	}
	for i := range q.User.PullRequests.Nodes {
		items = append(items, &q.User.PullRequests.Nodes[i])
	}
	for i := range q.User.RepositoryDiscussions.Nodes {
		items = append(items, &q.User.RepositoryDiscussions.Nodes[i])
	}
	for i := range q.User.RepositoryDiscussionComments.Nodes {
		items = append(items, &q.User.RepositoryDiscussionComments.Nodes[i])
	}
	sort.Slice(items, func(i, j int) bool { return items[i].GetTime().After(items[j].GetTime()) })
	for i := range items {
		// fmt.Printf("%T\t(%+v)\n", items[i], items[i])
		var count *int
		switch items[i].(type) {
		case *IssueNode:
			count = &counts.issues
		case *IssueCommentNode:
			count = &counts.issuecomments
		case *PullRequestNode:
			count = &counts.pullrequests
		case *RepositoryDiscussionNode:
			count = &counts.repositorydiscussions
		case *RepositoryDiscussionCommentNode:
			count = &counts.repositorydiscussioncomments
		}
		*count--
		if *count == 0 {
			items = items[:i]
			break
		}
	}

	// TODO: cursors

	t, err := template.ParseFS(templatefs, "*")
	if err != nil {
		log.Fatal(err)
	}
	b := new(bytes.Buffer)
	if err := t.Execute(b, items); err != nil {
		log.Fatal(err)
	}
	io.Copy(os.Stdout, b)
}

type Item interface {
	GetRepositoryNameWithOwner() string
	GetTime() time.Time
	GetTitle() string
	GetType() string
}
