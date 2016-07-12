-- Copyright 2015 Alex Browne.  All rights reserved.
-- Use of this source code is governed by the MIT
-- license, which can be found in the LICENSE file.

-- pop_next_jobs is a lua script that takes the following arguments:
-- 	1) The maximum number of jobs to pop and return
-- 	2) The current unix time UTC with nanosecond precision
-- The script gets the next available jobs from the queued set which are
-- ready based on their time parameter. Then it adds those jobs to the
-- executing set, sets their status to executing, and removes them from the
-- queued set. It returns an array of arrays where each element contains the
-- fields for a particular job, and the jobs are sorted by priority.
-- Here's an example response:
-- [
-- 	[
-- 		"id", "afj9afjpa30",
-- 		"data", [34, 67, 34, 23, 56, 67, 78, 79],
-- 		"type", "emailJob",
-- 		"time", 1234567,
-- 		"freq", 0,
-- 		"priority", 100,
-- 		"retries", 0,
-- 		"status", "executing",
-- 		"started", 0,
-- 		"finished", 0,
-- 	],
-- 	[
-- 		"id", "E8v2ovkdaIw",
-- 		"data", [46, 43, 12, 08, 34, 45, 57, 43],
-- 		"type", "emailJob",
-- 		"time", 1234568,
-- 		"freq", 0,
-- 		"priority", 95,
-- 		"retries", 0,
-- 		"status", "executing",
-- 		"started", 0,
-- 		"finished", 0,
-- 	]
-- ]

-- IMPORTANT: If you edit this file, you must run go generate . to rewrite ../scripts.go

-- Assign args to variables for easy reference
local n = ARGV[1]
local currentTime = ARGV[2]
local poolId = ARGV[3]
local poolKey = poolId:sub(18)

-- Copy the time index set to a new temporary set
redis.call('ZUNIONSTORE', '{{.jobsTempSet}}', 1, '{{.timeIndexSet}}')
-- Trim the new temporary set we just created to leave only the jobs which have a time
-- parameter in the past
redis.call('ZREMRANGEBYSCORE', '{{.jobsTempSet}}', currentTime, '+inf')
-- Intersect the jobs which are ready based on their time with those in the
-- queued set. Use the weights parameter to set the scores entirely based on the
-- queued set, effectively sorting the jobs by priority. Store the results in the
-- temporary set.

redis.call('ZINTERSTORE', '{{.jobsTempSet}}', 2, '{{.queuedSet}}'..poolKey, '{{.jobsTempSet}}', 'WEIGHTS', 1, 0)
-- Trim the temp set, so it contains only the first n jobs ordered by
-- priority
redis.call('ZREMRANGEBYRANK', '{{.jobsTempSet}}', 0, -n - 1)
-- Get all job ids from the temp set
local jobIds = redis.call('ZREVRANGE', '{{.jobsTempSet}}', 0, -1)
local allJobs = {}
if #jobIds > 0 then
	-- Add job ids to the executing set
	redis.call('ZUNIONSTORE', '{{.executingSet}}'..poolKey, 2, '{{.executingSet}}'..poolKey, '{{.jobsTempSet}}')
	-- Now we are ready to construct our response.
	for i, jobId in ipairs(jobIds) do
		local jobKey = 'jobs:' .. jobId
		-- Remove the job from the queued set
		redis.call('ZREM', '{{.queuedSet}}'..poolKey, jobId)
		-- Set the poolId field for the job
		redis.call('HSET', jobKey, 'poolId', poolId)
		-- Set the job status to executing
		redis.call('HSET', jobKey, 'status', '{{.statusExecuting}}')
		-- Get the fields from its main hash
		local jobFields = redis.call('HGETALL', jobKey)
		-- Add the id itself to the fields
		jobFields[#jobFields+1] = 'id'
		jobFields[#jobFields+1] = jobId
		-- Add the field values to allJobs
		allJobs[#allJobs+1] = jobFields
	end
end
-- Delete the temporary set
redis.call('DEL', '{{.jobsTempSet}}')
-- Return all the fields for all the jobs
return allJobs