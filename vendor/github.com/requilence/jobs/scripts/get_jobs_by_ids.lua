-- Copyright 2015 Alex Browne.  All rights reserved.
-- Use of this source code is governed by the MIT
-- license, which can be found in the LICENSE file.

-- get_jobs_by_ids is a lua script that takes the following arguments:
-- 	1) The key of a sorted set of some job ids
-- The script then gets all the data for those job ids from their respective
-- hashes in the database. It returns an array of arrays where each element
-- contains the fields for a particular job, and the jobs are sorted by
-- priority.
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

-- Assign keys to variables for easy access
local setKey = ARGV[1]
-- Get all the ids from the set name
local jobIds = redis.call('ZREVRANGE', setKey, 0, -1)
local allJobs = {}
if #jobIds > 0 then
	-- Iterate over the ids and find each job
	for i, jobId in ipairs(jobIds) do
		local jobKey = 'jobs:' .. jobId
		local jobFields = redis.call('HGETALL', jobKey)
		-- Add the id itself to the fields
		jobFields[#jobFields+1] = 'id'
		jobFields[#jobFields+1] = jobId
		-- Add the field values to allJobs
		allJobs[#allJobs+1] = jobFields
	end
end
return allJobs