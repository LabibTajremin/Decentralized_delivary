-- Atomically exchange one refresh token hash for another.
--
-- KEYS[1]  refresh:<old hash>
-- ARGV[1]  old hash
-- ARGV[2]  new hash
-- ARGV[3]  ttl in seconds
-- ARGV[4]  now, RFC3339
--
-- Returns {"ok", session_id} | {"reuse", session_id} | {"unknown"}
--
-- This is one script rather than a sequence of commands from Go because the
-- read and the delete must not be separable. With a gap between them, two
-- refreshes arriving together from the same device both see a valid token and
-- both mint a new one — or, worse, the second is treated as theft and signs an
-- innocent user out of every device.

local old_hash = ARGV[1]
local new_hash = ARGV[2]
local ttl      = tonumber(ARGV[3])
local now      = ARGV[4]

-- GETDEL: whoever removes the key is the one rotation that proceeds. A second
-- caller finds it already gone and falls through to the reuse check below.
local session_id = redis.call('GETDEL', KEYS[1])

if not session_id then
    -- The token is not current. Either it was never ours, or it has already
    -- been rotated away — and those two cases mean very different things.
    --
    -- Scanning every family would be O(sessions). Instead the hash itself is
    -- looked up in a spent-token index, which is O(1).
    local spent_owner = redis.call('GET', 'refresh_spent:' .. old_hash)
    if spent_owner then
        return {'reuse', spent_owner}
    end
    return {'unknown'}
end

-- The session must still exist. A refresh token whose session has expired or
-- been revoked is not a valid credential, whatever its own TTL says.
local session_key = 'session:' .. session_id
if redis.call('EXISTS', session_key) == 0 then
    return {'unknown'}
end

-- Record the old hash as spent, so presenting it again is recognised as reuse
-- rather than as an unknown token. It is kept for the session's lifetime: a
-- thief who copied a token will present it long after it was rotated away, and
-- an index that forgets sooner would classify the theft as a stale token.
redis.call('SETEX', 'refresh_spent:' .. old_hash, ttl, session_id)

-- Install the replacement.
redis.call('SETEX', 'refresh:' .. new_hash, ttl, session_id)
redis.call('SADD', 'refresh_family:' .. session_id, new_hash)
redis.call('EXPIRE', 'refresh_family:' .. session_id, ttl)

-- Sliding expiry: an app in daily use stays signed in indefinitely, while one
-- abandoned for sixty days does not.
redis.call('HSET', session_key, 'last_seen_at', now)
redis.call('EXPIRE', session_key, ttl)

return {'ok', session_id}
