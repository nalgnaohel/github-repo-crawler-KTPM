package entity

type Commit struct {
	ID        int64   `gorm:"column:id;primaryKey"`
	Hash      string  `gorm:"column:hash"`
	Message   string  `gorm:"column:message"`
	ReleaseID int64   `gorm:"column:releaseid"`
	Release   Release `gorm:"foreignKey:releaseid;references:id"`
}
