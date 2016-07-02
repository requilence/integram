-- Copyright 2015 Alex Browne.  All rights reserved.
-- Use of this source code is governed by the MIT
-- license, which can be found in the LICENSE file.

-- set_job_field represents a lua script that takes the following arguments:
-- 	1) The id of the job
--    2) The name of the field
--    3) The value to set the field to
-- It first checks if the job exists in the database (has not been destroyed)
-- and then sets the given field to the given value.

-- IMPORTANT: If you edit this file, you must run go generate . to rewrite ../scripts.go

local jobId = ARGV[1]
local fieldName = ARGV[2]
local fieldVal = ARGV[3]
local jobKey = 'jobs:' .. jobId
-- Make sure the job hasn't already been destroyed
local exists = redis.call('EXISTS', jobKey)
if exists ~= 1 then
	return
end
redis.call('HSET', jobKey, fieldName, fieldVal)