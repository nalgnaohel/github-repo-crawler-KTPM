package model

type RepoResponse struct {
	ID       int64  `json:"id,omitempty"`
	UserName string `json:"userName,omitempty"`
	RepoName string `json:"repoName,omitempty"`
}

type CreateRepoRequest struct {
	RepoName string `json:"repoName" validate:"required"`
	UserName string `json:"userName" validate:"required"`
}

type SearchRepoRequest struct {
	RepoName string `json:"repoName" validate:"required"`
	UserName string `json:"userName" validate:"required"`
}
