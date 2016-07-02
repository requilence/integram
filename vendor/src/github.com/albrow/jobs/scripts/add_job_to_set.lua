-- Copyright 2015 Alex Browne.  All rights reserved.
-- Use of this source code is governed by the MIT
-- license, which can be found in the LICENSE file.

-- add_job_to_set represents a lua script that takes the following arguments:
-- 	1) The id of the job
--    2) The name of a sorted set
--    3) The score the inserted job should have in the sorted set
-- It first checks if the job exists in the database (has not been destroyed)
-- and then adds it to the sorted set with the given score.

-- IMPORTANT: If you edit this file, you must run go generate . to rewrite ../scripts.go

local jobId = ARGV[1]
local setName = ARGV[2]
local score = ARGV[3]
local jobKey = 'jobs:' .. jobId
-- Make sure the job hasn't already been destroyed
local exists = redis.call('EXISTS', jobKey)
if exists ~= 1 then
	return
end
redis.call('ZADD', setName, score, jobId)