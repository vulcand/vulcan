package vulcan

import (
	"encoding/json"
	"fmt"
	"time"
)

// Rates stores the information on how many hits per
// period of time any endpoint can accept
type Rate struct {
	Value  int
	Period time.Duration
}

// This object is used for unmarshalling
// from json
type RateObj struct {
	value  int
	period string
}

func NewRate(value int, period string) (*Rate, error) {
	duration, err := periodDuration(period)
	if err != nil {
		return nil, err
	}
	return &Rate{Value: value, Period: duration}, nil
}

func rateFromJson(bytes []byte) (*Rate, error) {
	var r RateObj
	err := json.Unmarshal(bytes, &r)
	if err != nil {
		return nil, err
	}
	return NewRate(r.value, r.period)
}

func ratesFromJsonList(inRates []RateObj) ([]Rate, error) {
	rates := make([]Rate, len(inRates))
	for i, rateObj := range inRates {
		rate, err := NewRate(rateObj.value, rateObj.period)
		if err != nil {
			return nil, err
		}
		rates[i] = *rate
	}
	return rates, nil
}

func periodDuration(period string) (time.Duration, error) {
	switch period {
	case "second":
		return time.Second, nil
	case "minute":
		return time.Minute, nil
	case "hour":
		return time.Hour, nil
	case "day":
		return 24 * time.Hour, nil
	}
	return -1, fmt.Errorf("Unsupported period: %s", period)
}
