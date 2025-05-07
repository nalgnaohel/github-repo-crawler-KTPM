package entity

type Release struct {
	ID         int64      `gorm:"column:id;primaryKey"`
	TagName    string     `gorm:"column:tagname"`
	Content    string     `gorm:"column:content"`
	RepoID     int64      `gorm:"column:repoid"`
	Repository Repository `gorm:"foreignKey:repoid;references:id"`
	Commits    []Commit   `gorm:"foreignKey:releaseid;references:id"`
}
