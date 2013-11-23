package ratelimit

import (
	"github.com/golang/glog"
	"github.com/mailgun/vulcan/backend"
	"github.com/mailgun/vulcan/command"
)

type RateLimiter struct {
	Backend backend.Backend
}

func (rl *RateLimiter) GetRetrySeconds(rates map[string][]*command.Rate) (retrySeconds int, err error) {
	for key, rateList := range rates {
		for _, rate := range rateList {
			counter, err := rl.Backend.GetCount(key, rate.Period)
			if err != nil {
				return 0, err
			}
			if counter > rate.Units {
				glog.Infof("Key('%s') %v is out of capacity", key, rate)
				return rate.RetrySeconds(rl.Backend.UtcNow()), nil
			}
		}
	}
	return 0, nil
}

func (rl *RateLimiter) UpdateStats(requestBytes int64, rates map[string][]*command.Rate) error {
	for key, rateList := range rates {
		for _, rate := range rateList {
			err := rl.Backend.UpdateCount(key, rate.Period, getCount(requestBytes, rate))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func getCount(requestBytes int64, rate *command.Rate) int64 {
	switch rate.UnitType {
	case command.UnitTypeRequests:
		return 1
	case command.UnitTypeKilobytes:
		return requestBytes / 1024
	}
	return 1
}
