package core

type Symbol string

const UnknownSymbol Symbol = ""

func (s Symbol) Valid() bool {
	return s != UnknownSymbol
}

func (s Symbol) String() string {
	return string(s)
}
