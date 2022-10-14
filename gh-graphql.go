package main

import (
	"embed"
	"sort"
	"time"

	"github.com/shurcooL/githubv4"
)

type Q struct {
	User struct {
		Login  githubv4.String
		Issues struct {
			Nodes []IssueNode
			// PageInfo PageInfo
		} `graphql:"issues(first: $first, after: $issuescursor, orderBy:{direction:DESC, field:CREATED_AT})"`
		IssueComments struct {
			Nodes []IssueCommentNode
			// PageInfo PageInfo
		} `graphql:"issueComments(first: $first, after: $issuecommentscursor, orderBy:{direction:DESC, field:UPDATED_AT})"`
		PullRequests struct {
			Nodes []PullRequestNode
			// PageInfo PageInfo
		} `graphql:"pullRequests(first: $first, after: $pullrequestscursor, orderBy:{direction:DESC, field:CREATED_AT})"`
		RepositoryDiscussions struct {
			Nodes []RepositoryDiscussionNode
			// PageInfo PageInfo
		} `graphql:"repositoryDiscussions(first: $first, after: $repositorydiscussionscursor, orderBy:{direction:DESC, field:CREATED_AT})"`
		RepositoryDiscussionComments struct {
			Nodes []RepositoryDiscussionCommentNode
			// PageInfo PageInfo
		} `graphql:"repositoryDiscussionComments(first: $first, after: $repositorydiscussioncommentscursor)"`
	} `graphql:"user(login: $login)"`
}

func (q *Q) Items() []Item {
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
	return items
}

type Item interface {
	GetRepositoryNameWithOwner() string
	GetTime() time.Time
	GetTitle() string
	GetType() string
}

type IssueNode struct {
	BodyText       githubv4.String
	CreatedAt      githubv4.DateTime
	Title          githubv4.String
	URL            githubv4.String
	ReactionGroups ReactionsGroups
	Repository     struct {
		NameWithOwner githubv4.String
	}
}

func (n *IssueNode) GetRepositoryNameWithOwner() string { return string(n.Repository.NameWithOwner) }
func (n *IssueNode) GetTime() time.Time                 { return n.CreatedAt.Time }
func (n *IssueNode) GetTitle() string                   { return string(n.Title) }
func (n *IssueNode) GetType() string                    { return "IS" }

type IssueCommentNode struct {
	BodyText  githubv4.String
	UpdatedAt githubv4.DateTime
	URL       githubv4.String
	Issue     struct {
		Title githubv4.String
	}
	ReactionGroups ReactionsGroups
	Repository     struct {
		NameWithOwner githubv4.String
	}
}

func (n *IssueCommentNode) GetRepositoryNameWithOwner() string {
	return string(n.Repository.NameWithOwner)
}
func (n *IssueCommentNode) GetTime() time.Time { return n.UpdatedAt.Time }
func (n *IssueCommentNode) GetTitle() string   { return string(n.Issue.Title) }
func (n *IssueCommentNode) GetType() string    { return "IC" }

type PullRequestNode struct {
	BodyText       githubv4.String
	CreatedAt      githubv4.DateTime
	Title          githubv4.String
	URL            githubv4.String
	ReactionGroups ReactionsGroups
	Repository     struct {
		NameWithOwner githubv4.String
	}
}

func (n *PullRequestNode) GetRepositoryNameWithOwner() string {
	return string(n.Repository.NameWithOwner)
}
func (n *PullRequestNode) GetTime() time.Time { return n.CreatedAt.Time }
func (n *PullRequestNode) GetTitle() string   { return string(n.Title) }
func (n *PullRequestNode) GetType() string    { return "PR" }

type RepositoryDiscussionNode struct {
	BodyText       githubv4.String
	CreatedAt      githubv4.DateTime
	Title          githubv4.String
	URL            githubv4.String
	ReactionGroups ReactionsGroups
	Repository     struct {
		NameWithOwner githubv4.String
	}
}

func (n *RepositoryDiscussionNode) GetRepositoryNameWithOwner() string {
	return string(n.Repository.NameWithOwner)
}
func (n *RepositoryDiscussionNode) GetTime() time.Time { return n.CreatedAt.Time }
func (n *RepositoryDiscussionNode) GetTitle() string   { return string(n.Title) }
func (n *RepositoryDiscussionNode) GetType() string    { return "DI" }

type RepositoryDiscussionCommentNode struct {
	BodyText   githubv4.String
	CreatedAt  githubv4.DateTime
	URL        githubv4.String
	Discussion struct {
		Repository struct {
			NameWithOwner githubv4.String
		}
		Title githubv4.String
	}
	ReactionGroups ReactionsGroups
}

func (n *RepositoryDiscussionCommentNode) GetRepositoryNameWithOwner() string {
	return string(n.Discussion.Repository.NameWithOwner)
}
func (n *RepositoryDiscussionCommentNode) GetTime() time.Time { return n.CreatedAt.Time }
func (n *RepositoryDiscussionCommentNode) GetTitle() string   { return string(n.Discussion.Title) }
func (n *RepositoryDiscussionCommentNode) GetType() string    { return "DC" }

type ReactionsGroups []struct {
	Content  githubv4.String
	Reactors struct {
		TotalCount githubv4.Int
	}
}
type PageInfo struct {
	EndCursor   githubv4.String
	HasNextPage githubv4.Boolean
}

//go:embed *.html
var templatefs embed.FS
