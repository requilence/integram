-- Copyright 2015 Alex Browne.  All rights reserved.
-- Use of this source code is governed by the MIT
-- license, which can be found in the LICENSE file.

-- purge_stale_pool is a lua script which takes the following arguments:
-- 	1) The id of the stale pool to purge
-- It then does the following:
-- 	1) Removes the pool id from the set of active pools
-- 	2) Iterates through each job in the executing set and finds any jobs which
-- 		have a poolId field equal to the id of the stale pool
-- 	3) If it finds any such jobs, it removes them from the executing set and
-- 		adds them to the queued so that they will be retried

-- IMPORTANT: If you edit this file, you must run go generate . to rewrite ../scripts.go

-- Assign args to variables for easy reference
local stalePoolId = ARGV[1]
local poolKey = stalePoolId:sub(18)

-- Check if the stale pool is in the set of active pools first
local isActive = redis.call('SISMEMBER', '{{.activePoolsSet}}', stalePoolId)
if isActive then
	-- Remove the stale pool from the set of active pools
	redis.call('SREM', '{{.activePoolsSet}}', stalePoolId)
	-- Get all the jobs in the executing set
	local jobIds = redis.call('ZRANGE', '{{.executingSet}}'..poolKey, 0, -1)
	for i, jobId in ipairs(jobIds) do
		local jobKey = 'jobs:' .. jobId
		-- Check the poolId field
		-- If the poolId is equal to the stale id, then this job is stuck
		-- in the executing set even though no worker is actually executing it
		local poolId = redis.call('HGET', jobKey, 'poolId')
		if poolId == stalePoolId then
			local jobPriority = redis.call('HGET', jobKey, 'priority')
			-- Move the job into the queued set
			redis.call('ZADD', '{{.queuedSet}}'..poolKey, jobPriority, jobId)
			-- Remove the job from the executing set
			redis.call('ZREM', '{{.executingSet}}'..poolKey, jobId)
			-- Set the job status to queued and the pool id to blank
			redis.call('HMSET', jobKey, 'status', '{{.statusQueued}}', 'poolId', '')
		end
	end
end
