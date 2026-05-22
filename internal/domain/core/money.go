package core

import "github.com/shopspring/decimal"

type Money struct {
	decimal.Decimal
}

func NewMoney(value string) (Money, error) {
	d, err := decimal.NewFromString(value)
	if err != nil {
		return Money{}, err
	}
	return Money{d}, nil
}

func NewMoneyFromInt(value int64) Money {
	return Money{decimal.NewFromInt(value)}
}

func NewMoneyFromFloat(value float64) Money {
	return Money{decimal.NewFromFloat(value)}
}

func (m Money) Add(other Money) Money {
	return Money{m.Decimal.Add(other.Decimal)}
}

func (m Money) Sub(other Money) Money {
	return Money{m.Decimal.Sub(other.Decimal)}
}

func (m Money) Mul(n int64) Money {
	return Money{m.Decimal.Mul(decimal.NewFromInt(n))}
}

func (m Money) Div(n int64) Money {
	return Money{m.Decimal.Div(decimal.NewFromInt(n))}
}

func (m Money) GreaterThan(other Money) bool {
	return m.Decimal.GreaterThan(other.Decimal)
}

func (m Money) GreaterThanOrEqual(other Money) bool {
	return m.Decimal.GreaterThanOrEqual(other.Decimal)
}

func (m Money) LessThan(other Money) bool {
	return m.Decimal.LessThan(other.Decimal)
}

func (m Money) LessThanOrEqual(other Money) bool {
	return m.Decimal.LessThanOrEqual(other.Decimal)
}

func (m Money) IsZero() bool {
	return m.Decimal.IsZero()
}

func (m Money) IsNegative() bool {
	return m.Decimal.IsNegative()
}

func (m Money) String() string {
	return m.Decimal.StringFixed(2)
}
