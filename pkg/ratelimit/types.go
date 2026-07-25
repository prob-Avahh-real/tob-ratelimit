package ratelimit

import "github.com/skat/tob-ratelimit/internal/model"

type Tier = model.Tier

const (
	TierBlock     = model.TierBlock
	TierCrawler   = model.TierCrawler
	TierAnonymous = model.TierAnonymous
	TierUser      = model.TierUser
	TierEndpoint  = model.TierEndpoint
)

type Identity = model.Identity
type Result = model.Result
