package model

type CommitResponse struct {
	ID        int64  `json:"id"`
	Hash      string `json:"hash"`
	Message   string `json:"message"`
	ReleaseID int64  `json:"releaseID"`
}

type CreateCommitRequest struct {
	Hash      string `json:"hash"`
	Message   string `json:"message"`
	ReleaseID int64  `json:"releaseID"`
}

type CommitData struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
}
