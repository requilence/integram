-- Copyright 2015 Alex Browne.  All rights reserved.
-- Use of this source code is governed by the MIT
-- license, which can be found in the LICENSE file.

-- retry_or_fail_job represents a lua script that takes the following arguments:
-- 	1) The id of the job to either retry or fail
-- It first checks if the job has any retries remaining. If it does,
-- then it:
-- 	1) Decrements the number of retries for the given job
-- 	2) Adds the job to the queued set
-- 	3) Removes the job from the executing set
-- 	4) Returns true
-- If the job has no retries remaining then it:
-- 	1) Adds the job to the failed set
-- 	3) Removes the job from the executing set
-- 	2) Returns false

-- IMPORTANT: If you edit this file, you must run go generate . to rewrite ../scripts.go

-- Assign args to variables for easy reference

local function Fibonacci(n)
	local function inner(m, a, b)
		if m == 0 then
			return a
		end
		return inner(m-1, b, a+b)
	end
	return inner(n, 0, 1)
end


local jobId = ARGV[1]
local currentTime = tonumber(ARGV[2])

local function NextRetryTime(n)
	return string.format("%.0f",(currentTime+Fibonacci(n)*1000000000))
end

redis.log(redis.LOG_WARNING, "jobID: "..jobId)

local jobKey = 'jobs:' .. jobId
-- Make sure the job hasn't already been destroyed
local exists = redis.call('EXISTS', jobKey)
if exists ~= 1 then
	return 0
end
-- Check how many retries remain
local retries = redis.call('HGET', jobKey, 'retries')
redis.log(redis.LOG_WARNING, "retries left: "..retries)

local poolKey = redis.call('HGET', jobKey, 'poolKey')

local newStatus = ''
if retries == '0' then
	redis.log(redis.LOG_WARNING, "no retries left...")
	-- newStatus should be failed because there are no retries left
	newStatus = '{{.statusFailed}}'
else
	local totalRetries = redis.call('HGET', jobKey, 'totalRetries')
	local nextTime = NextRetryTime(totalRetries-retries+1)

	redis.call('HINCRBY', jobKey, 'retries', -1)
	redis.call('HSET', jobKey, 'time', nextTime)
	redis.pcall('ZADD', '{{.timeIndexSet}}', nextTime, jobId)
	-- subtract 1 from the remaining retries
	-- newStatus should be queued, so the job will be retried
	newStatus = '{{.statusQueued}}'
end
-- Get the job priority (used as score)
local jobPriority = redis.call('HGET', jobKey, 'priority')
-- Add the job to the appropriate new set
local newStatusSet = 'jobs:' .. newStatus..poolKey
redis.call('ZADD', newStatusSet, jobPriority, jobId)
-- Remove the job from the old status set
local oldStatus = redis.call('HGET', jobKey, 'status')
if ((oldStatus ~= '') and (oldStatus ~= newStatus)) then
	local oldStatusSet = 'jobs:' .. oldStatus
	redis.call('ZREM', oldStatusSet, jobId)
end
-- Set the job status in the hash
redis.call('HSET', jobKey, 'status', newStatus)
return retries
