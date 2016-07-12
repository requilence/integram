# xxhash
	
xxhash is a pure go (golang) implementation of [xxhash](http://code.google.com/p/xxhash/).

## Benchmark

```go test github.com/vova616/xxhash -bench=".*"```

Core i7-3770K CPU @ 3.50GHz
go version devel +16e0e01c2e9b Sat Mar 09 18:14:00 2013 -0800 windows/386
	
```
Benchmark_xxhash32     			50000000     61.1 ns/op
Benchmark_CRC32IEEE    			10000000      145 ns/op
Benchmark_Adler32      	 		10000000      181 ns/op
Benchmark_Fnv32 				10000000      162 ns/op
Benchmark_MurmurHash3Hash32     1000000      1927 ns/op
```

# Note:

The package uses unsafe to get higher performance its safe as far as I know but if you don't want it you can use switch to early commits.