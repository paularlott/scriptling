package ai

import "sync/atomic"

// maxParallelLimit is the host-set ceiling on the max_parallel kwarg accepted
// by Pipeline, completion_parallel and ask_parallel. 0 means no ceiling.
var maxParallelLimit atomic.Int64

// SetMaxParallelLimit caps the max_parallel a script may request from
// client.Pipeline, client.completion_parallel and client.ask_parallel. A
// script asking for more than the ceiling silently gets the ceiling; it is a
// resource policy for the host, not an error for the script. A limit of 0
// removes the ceiling; negative values are treated as 0. The check is one
// atomic load when a pipeline is created, so it costs nothing per request.
func SetMaxParallelLimit(n int) {
	if n < 0 {
		n = 0
	}
	maxParallelLimit.Store(int64(n))
}

// MaxParallelLimit returns the current ceiling on max_parallel, 0 when none.
func MaxParallelLimit() int {
	return int(maxParallelLimit.Load())
}

// clampMaxParallel normalises a requested max_parallel: at least 1, and no
// more than the host ceiling when one is set.
func clampMaxParallel(requested int) int {
	if requested < 1 {
		requested = 1
	}
	if limit := int(maxParallelLimit.Load()); limit > 0 && requested > limit {
		return limit
	}
	return requested
}
