package vulcan

type Token struct {
	Id    string
	Rates []*Rate
}

func NewToken(id string, rates []*Rate) (*Token, error) {
	return &Token{
		Id:    id,
		Rates: rates,
	}, nil
}
