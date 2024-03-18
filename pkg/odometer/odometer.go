package odometer

import (
	"fmt"
	"strings"

	"github.com/dkforest/pkg/utils"
)

type Odometer struct {
	s string
}

func New(s string) (*Odometer, error) {
	if s == "" {
		return nil, fmt.Errorf("input string cannot be empty")
	}
	if len(s) > 9 {
		return nil, fmt.Errorf("input string cannot be longer than 9 characters")
	}
	return &Odometer{s: s}, nil
}

func (o *Odometer) SetValue(s string) error {
	if s == "" {
		return fmt.Errorf("input string cannot be empty")
	}
	if len(s) > 9 {
		return fmt.Errorf
