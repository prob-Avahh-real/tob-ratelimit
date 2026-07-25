package ratelimit

import "github.com/skat/tob-ratelimit/internal/model"

type BucketParams = model.BucketParams
type EndpointLimit = model.EndpointLimit
type Config = model.Config

func DefaultConfig() Config {
	return model.DefaultConfig()
}
