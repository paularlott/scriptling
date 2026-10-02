package extlibs

import "sync/atomic"

// requestsMaxParallelLimit is the host-set ceiling on the max_parallel kwarg
// accepted by requests.parallel. 0 means no ceiling.
var requestsMaxParallelLimit atomic.Int64

// SetRequestsMaxParallelLimit caps the max_parallel a script may request from
// requests.parallel. A script asking for more than the ceiling silently gets
// the ceiling; it is a resource policy for the host, not an error for the
// script. A limit of 0 removes the ceiling; negative values are treated as 0.
// The check is one atomic load per parallel() call.
func SetRequestsMaxParallelLimit(n int) {
	if n < 0 {
		n = 0
	}
	requestsMaxParallelLimit.Store(int64(n))
}

// RequestsMaxParallelLimit returns the current ceiling on requests.parallel's
// max_parallel, 0 when none.
func RequestsMaxParallelLimit() int {
	return int(requestsMaxParallelLimit.Load())
}

// clampRequestsMaxParallel normalises a requested max_parallel: at least 1,
// and no more than the host ceiling when one is set.
func clampRequestsMaxParallel(requested int64) int64 {
	if requested < 1 {
		requested = 1
	}
	if limit := requestsMaxParallelLimit.Load(); limit > 0 && requested > limit {
		return limit
	}
	return requested
}
