package bucket

// tokenBucketLua atomically refills and consumes tokens.
// KEYS[1] = bucket key
// ARGV[1] = capacity, ARGV[2] = rate/sec, ARGV[3] = weight, ARGV[4] = cost, ARGV[5] = now_ms
const tokenBucketLua = `
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local weight = tonumber(ARGV[3])
local cost = tonumber(ARGV[4])
local now_ms = tonumber(ARGV[5])

local data = redis.call('HMGET', key, 'tokens', 'ts')
local tokens = tonumber(data[1])
local ts = tonumber(data[2])

if tokens == nil then
  tokens = capacity
  ts = now_ms
else
  local elapsed = math.max(0, now_ms - ts) / 1000.0
  tokens = math.min(capacity, tokens + elapsed * rate * weight)
  ts = now_ms
end

local allowed = 0
local retry_ms = 0
if tokens >= cost then
  tokens = tokens - cost
  allowed = 1
else
  local deficit = cost - tokens
  if rate * weight > 0 then
    retry_ms = math.ceil(deficit / (rate * weight) * 1000)
  else
    retry_ms = 60000
  end
end

redis.call('HMSET', key, 'tokens', tokens, 'ts', ts)
redis.call('PEXPIRE', key, math.ceil((capacity / (rate * weight + 0.0001)) * 2000))

return {allowed, tokens, retry_ms}
`
