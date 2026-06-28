package validation

import (
	"errors"
	"fmt"
	"strings"

	"warband-vault/internal/campaign"
	"warband-vault/internal/character"
)

var ErrInvalid = errors.New("invalid input")

type FieldError struct {
	Field   string
	Message string
}

type ErrorList []FieldError

func (e ErrorList) Error() string {
	parts := make([]string, 0, len(e))
	for _, field := range e {
		parts = append(parts, field.Field+": "+field.Message)
	}
	return strings.Join(parts, "; ")
}

func (e ErrorList) Unwrap() error {
	return ErrInvalid
}

func ValidateCampaign(c *campaign.Campaign) error {
	var errs ErrorList
	if c == nil {
		return fmt.Errorf("%w: campaign is nil", ErrInvalid)
	}
	if strings.TrimSpace(c.Name) == "" {
		errs = append(errs, FieldError{Field: "name", Message: "cannot be blank"})
	}
	if c.Treasury < 0 {
		errs = append(errs, FieldError{Field: "treasury", Message: "cannot be negative"})
	}
	for i := range c.Characters {
		if err := ValidateCharacter(&c.Characters[i]); err != nil {
			errs = append(errs, FieldError{Field: fmt.Sprintf("characters[%d]", i), Message: err.Error()})
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func ValidateCharacter(c *character.Character) error {
	var errs ErrorList
	if c == nil {
		return fmt.Errorf("%w: character is nil", ErrInvalid)
	}
	if strings.TrimSpace(c.Name) == "" {
		errs = append(errs, FieldError{Field: "name", Message: "cannot be blank"})
	}
	if c.Level < 0 {
		errs = append(errs, FieldError{Field: "level", Message: "cannot be negative"})
	}
	if c.Experience < 0 {
		errs = append(errs, FieldError{Field: "experience", Message: "cannot be negative"})
	}
	if c.Health < 0 {
		errs = append(errs, FieldError{Field: "health", Message: "cannot be negative"})
	}
	if c.Movement < 0 {
		errs = append(errs, FieldError{Field: "movement", Message: "cannot be negative"})
	}
	if c.Armor < 0 {
		errs = append(errs, FieldError{Field: "armor", Message: "cannot be negative"})
	}
	for key := range c.CustomFields {
		if strings.TrimSpace(key) == "" {
			errs = append(errs, FieldError{Field: "custom_fields", Message: "keys cannot be blank"})
		}
	}
	for i, item := range c.Equipment {
		if strings.TrimSpace(item.Name) == "" {
			errs = append(errs, FieldError{Field: fmt.Sprintf("equipment[%d].name", i), Message: "cannot be blank"})
		}
		if item.Quantity < 0 {
			errs = append(errs, FieldError{Field: fmt.Sprintf("equipment[%d].quantity", i), Message: "cannot be negative"})
		}
	}
	for i, trait := range c.Traits {
		if strings.TrimSpace(trait.Name) == "" {
			errs = append(errs, FieldError{Field: fmt.Sprintf("traits[%d].name", i), Message: "cannot be blank"})
		}
	}
	for i, injury := range c.Injuries {
		if strings.TrimSpace(injury.Name) == "" {
			errs = append(errs, FieldError{Field: fmt.Sprintf("injuries[%d].name", i), Message: "cannot be blank"})
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}
