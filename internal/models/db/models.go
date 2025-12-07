package db

import "time"

// User represents a user in the database
type User struct {
	ID        string    `db:"id" json:"id"`
	Username  string    `db:"username" json:"username"`
	Email     string    `db:"email" json:"email"`
	Password  string    `db:"password" json:"-"`
	Roles     []string  `db:"roles" json:"roles"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// Organisation represents an organization in the database
type Organisation struct {
	ID            string    `db:"id" json:"id"`
	Name          string    `db:"name" json:"name"`
	Description   string    `db:"description" json:"description"`
	OwnerID       string    `db:"owner_id" json:"owner_id"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`
	Status        string    `db:"status" json:"status"`
	MembersCount  int       `db:"members_count" json:"members_count"`
	ProjectsCount int       `db:"projects_count" json:"projects_count"`
	Settings      string    `db:"settings" json:"settings"`
}

// Project represents a project in the database
type Project struct {
	ID           string    `db:"id" json:"id"`
	Name         string    `db:"name" json:"name"`
	OrgID        string    `db:"org_id" json:"org_id"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
	Status       string    `db:"status" json:"status"`
	CORS         string    `db:"cors" json:"cors"`
	Capabilities string    `db:"capabilities" json:"capabilities"`
	Members      string    `db:"members" json:"members"`
}
