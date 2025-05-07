package entity

type Repository struct {
	ID       int64     `gorm:"column:id;primaryKey"`
	UserName string    `gorm:"column:username"`
	RepoName string    `gorm:"column:reponame"`
	Releases []Release `gorm:"foreignKey:repoid;references:id"`
}
