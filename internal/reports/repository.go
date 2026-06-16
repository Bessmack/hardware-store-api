package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/Bessmack/hardware-store-api/pkg/database"
)

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}
