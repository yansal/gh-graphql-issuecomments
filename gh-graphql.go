package main

import (
	"embed"
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
