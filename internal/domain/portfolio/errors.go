package portfolio

import "errors"

var (
	ErrInsufficientCash      = errors.New("insufficient cash")
	ErrPositionNotFound       = errors.New("position not found")
	ErrInsufficientPosition   = errors.New("insufficient position quantity")
	ErrDuplicatePosition      = errors.New("position already exists")
)
