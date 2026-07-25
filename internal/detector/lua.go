package detector

// slidingWindowLua counts events in a sliding time window using a sorted set.
// KEYS[1] = batch counter key
// ARGV[1] = now_ms, ARGV[2] = window_ms, ARGV[3] = ttl_ms
// Returns the number of events in the current window (including this call).
const slidingWindowLua = `
local key = KEYS[1]
local now_ms = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local ttl_ms = tonumber(ARGV[3])

local window_start = now_ms - window_ms

redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)

local seq = redis.call('INCR', key .. ':seq')
local member = now_ms .. '-' .. seq
redis.call('ZADD', key, now_ms, member)

local count = redis.call('ZCARD', key)

redis.call('PEXPIRE', key, ttl_ms)
redis.call('PEXPIRE', key .. ':seq', ttl_ms)

return count
`
