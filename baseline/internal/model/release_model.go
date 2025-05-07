package model

type ReleaseResponse struct {
	ID      int64            `json:"id,omitempty"`
	TagName string           `json:"tagName,omitempty"`
	Content string           `json:"content,omitempty"`
	RepoID  int64            `json:"repoID,omitempty"`
	Commits []CommitResponse `json:"commits,omitempty"`
}

type CreateReleaseRequest struct {
	Content string `json:"content" validate:"required"`
	RepoID  int64  `json:"repoID" validate:"required"`
	TagName string `json:"tagName" validate:"required"`
}
