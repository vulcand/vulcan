package vulcan

import (
	"fmt"
	"github.com/mailgun/vulcan/instructions"
	"time"
)

func getHit(now time.Time, key string, rate *instructions.Rate) string {
	return fmt.Sprintf(
		"%s_%s_%d", key, rate.Period.String(), rate.CurrentBucket(now).Unix())
}
