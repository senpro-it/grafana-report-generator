package models

type Dashboard struct {
	ID          int64
	UID         string
	Title       string
	FolderTitle string
	Slug        string
	OrgID       int64
	OrgName     string
}

type DetailedDashboard struct {
	Dashboard
	Variables map[string]string
}