package vulcan

import (
	"encoding/json"
)

// This object is used for unmarshalling
// from json
type TokenObj struct {
	Id    string
	Rates []RateObj
}

type Token struct {
	Id    string
	Rates []Rate
}

func NewToken(id string, rates []Rate) (*Token, error) {
	return &Token{
		Id:    id,
		Rates: rates,
	}, nil
}

func tokenFromJson(bytes []byte) (*Token, error) {
	var t TokenObj
	err := json.Unmarshal(bytes, &t)
	if err != nil {
		return nil, err
	}

	rates, err := ratesFromJsonList(t.Rates)
	if err != nil {
		return nil, err
	}

	return NewToken(t.Id, rates)
}

func tokensFromJsonList(inTokens []TokenObj) ([]Token, error) {
	tokens := make([]Token, len(inTokens))
	for i, tokenObj := range inTokens {
		rates, err := ratesFromJsonList(tokenObj.Rates)
		if err != nil {
			return nil, err
		}
		token, err := NewToken(tokenObj.Id, rates)
		if err != nil {
			return nil, err
		}
		tokens[i] = *token
	}
	return tokens, nil
}
