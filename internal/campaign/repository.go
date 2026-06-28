package campaign

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("campaign not found")

type Repository interface {
	Create(context.Context, *Campaign) error
	Update(context.Context, *Campaign) error
	Delete(context.Context, string) error
	FindByID(context.Context, string) (*Campaign, error)
	List(context.Context, bool) ([]Campaign, error)
}
