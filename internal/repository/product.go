package repository

import (
	"time"
)

type Products struct {
	ProductName string
	MakerName   string
	Price       int
	Reason      string
	URL         string
	ASIN        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Status      bool
}
