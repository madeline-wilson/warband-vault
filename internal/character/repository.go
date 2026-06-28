package character

import "context"

type Repository interface {
	Create(context.Context, *Character) error
	Update(context.Context, *Character) error
	Delete(context.Context, string) error
	FindByID(context.Context, string) (*Character, error)
	ListByCampaign(context.Context, string) ([]Character, error)
}
