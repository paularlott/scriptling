package ai

import (
	"context"
	"sync"
	"time"

	"github.com/paularlott/scriptling/object"
)

// pipelineItem is a single unit of work queued into the pipeline.
type pipelineItem struct {
	index   int
	message any
}

// pipelineRateLimitPause is how long every worker waits before starting new
// work after a completion reports a rate limit.
const pipelineRateLimitPause = time.Second

// pipelineLimiter is the adaptive concurrency controller shared by a
// pipeline's workers. It is additive-increase / multiplicative-decrease:
//
//   - a completion that hit a rate limit halves the limit (never below 1) and
//     pauses all workers for pipelineRateLimitPause;
//   - every run of `limit` consecutive completions without a rate limit raises
//     the limit by one, back up to the configured max_parallel.
//
// Without the recovery step a single 429 early in a long run would leave the
// pipeline crawling at reduced concurrency for the rest of the job.
type pipelineLimiter struct {
	mu         sync.Mutex
	max        int       // configured max_parallel
	limit      int       // current concurrency limit, 1 <= limit <= max
	active     int       // completions in flight
	successes  int       // consecutive non-rate-limited completions since the last limit change
	pauseUntil time.Time // no new work starts before this instant

	// wake is signalled (non-blocking, capacity 1) whenever a slot may have
	// become free: a release or a limit increase. Waiters re-check the state
	// under mu after waking, and a waiter that acquires while slots remain
	// re-signals so the wake-up cascades to the next waiter.
	wake chan struct{}
}

func newPipelineLimiter(maxParallel int) *pipelineLimiter {
	if maxParallel < 1 {
		maxParallel = 1
	}
	return &pipelineLimiter{
		max:   maxParallel,
		limit: maxParallel,
		wake:  make(chan struct{}, 1),
	}
}

// signal wakes one waiter if any is blocked in acquire. Never blocks.
func (l *pipelineLimiter) signal() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// acquire blocks until a slot is free and no rate-limit pause is active, or
// ctx is done. Returns false when the context was cancelled.
func (l *pipelineLimiter) acquire(ctx context.Context) bool {
	for {
		if ctx.Err() != nil {
			return false
		}

		l.mu.Lock()
		pause := time.Until(l.pauseUntil)
		if pause <= 0 && l.active < l.limit {
			l.active++
			more := l.active < l.limit
			l.mu.Unlock()
			if more {
				l.signal()
			}
			return true
		}
		l.mu.Unlock()

		if pause > 0 {
			timer := time.NewTimer(pause)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return false
			}
			continue
		}

		select {
		case <-l.wake:
		case <-ctx.Done():
			return false
		}
	}
}

// release frees the slot held by a completion and feeds its outcome back into
// the controller. rateLimited reports whether the completion was rate limited
// (even if it eventually succeeded after retries); ok reports whether it
// produced a usable response rather than an error. Errors neither shrink nor
// grow the limit.
func (l *pipelineLimiter) release(rateLimited, ok bool) {
	l.mu.Lock()
	l.active--
	switch {
	case rateLimited:
		l.limit /= 2
		if l.limit < 1 {
			l.limit = 1
		}
		l.successes = 0
		l.pauseUntil = time.Now().Add(pipelineRateLimitPause)
	case ok && l.limit < l.max:
		l.successes++
		if l.successes >= l.limit {
			l.limit++
			l.successes = 0
		}
	}
	l.mu.Unlock()
	l.signal()
}

// snapshot returns the current limit and in-flight count (for tests).
func (l *pipelineLimiter) snapshot() (limit, active int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.limit, l.active
}

// responseRateLimited reports whether a completion result carries retry
// metadata saying a rate limit was hit along the way.
func responseRateLimited(res object.Object) bool {
	respMap, ok := resultAsMap(res)
	if !ok {
		return false
	}
	retry, ok := respMap["retry"].(map[string]any)
	if !ok {
		return false
	}
	hit, _ := retry["rate_limit_hit"].(bool)
	return hit
}

// PipelineInstance holds the state for a running pipeline.  Workers are
// started immediately at construction and drain the queue until it is closed
// by flush() / complete().
type PipelineInstance struct {
	aiInstance *object.Instance // the owning AI client instance
	ctx        context.Context
	model      string
	kwargs     object.Kwargs
	ask        bool // true → workers run completion then extract text

	// queue is the work channel; add()/enqueue() send here, workers receive.
	queue chan pipelineItem

	// mu protects nextIndex, results, and closed.
	mu        sync.Mutex
	nextIndex int
	results   []object.Object
	closed    bool

	// wg tracks in-flight items: Add(1) in enqueue(), Done() in worker.
	wg sync.WaitGroup

	// limiter is the adaptive concurrency controller shared by the workers.
	limiter *pipelineLimiter
}

var (
	pipelineClass     *object.Class
	pipelineClassOnce sync.Once
)

// GetPipelineClass returns the Pipeline class singleton.
func GetPipelineClass() *object.Class {
	pipelineClassOnce.Do(func() {
		pipelineClass = buildPipelineClass()
	})
	return pipelineClass
}

// newPipelineInstance creates a Pipeline and starts its worker goroutines.
func newPipelineInstance(
	aiInst *object.Instance,
	ctx context.Context,
	kwargs object.Kwargs,
	model string,
	ask bool,
	maxParallel int,
) *PipelineInstance {
	p := &PipelineInstance{
		aiInstance: aiInst,
		ctx:        ctx,
		model:      model,
		kwargs:     kwargs,
		ask:        ask,
		queue:      make(chan pipelineItem, 65536),
		limiter:    newPipelineLimiter(maxParallel),
	}

	for i := 0; i < p.limiter.max; i++ {
		go p.worker()
	}
	return p
}

// worker drains the queue until it is closed.
func (p *PipelineInstance) worker() {
	for item := range p.queue {
		if !p.limiter.acquire(p.ctx) {
			// Context was cancelled; store an error for this slot.
			p.mu.Lock()
			p.results[item.index] = &object.Error{Message: "context cancelled"}
			p.mu.Unlock()
			p.wg.Done()
			continue
		}

		// Always run completion so rate-limit retry metadata is preserved.
		res := completionMethod(p.aiInstance, p.ctx, p.kwargs, p.model, item.message)
		_, isErr := res.(*object.Error)

		// Adaptive rate limit: a 429 halves concurrency and pauses the
		// workers; a run of clean completions grows it back.
		p.limiter.release(responseRateLimited(res), !isErr)

		// For ask mode, convert the completion response to plain text.
		if p.ask && !isErr {
			res = extractTextFromResponse(res)
		}

		p.mu.Lock()
		p.results[item.index] = res
		p.mu.Unlock()
		p.wg.Done()
	}
}

// enqueue is the internal (Go-side) add used by completion_parallel / ask_parallel.
func (p *PipelineInstance) enqueue(message any) {
	p.mu.Lock()
	idx := p.nextIndex
	p.nextIndex++
	p.results = append(p.results, nil)
	p.wg.Add(1)
	p.mu.Unlock()
	p.queue <- pipelineItem{index: idx, message: message}
}

// flush is the internal (Go-side) complete used by completion_parallel / ask_parallel.
func (p *PipelineInstance) flush(ctx context.Context) object.Object {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	close(p.queue)
	object.RunBlocking(ctx, func() { p.wg.Wait() })
	p.mu.Lock()
	results := p.results
	p.mu.Unlock()
	return &object.List{Elements: results}
}

// ---------------------------------------------------------------------------
// Scriptling method implementations
// ---------------------------------------------------------------------------

// addMethod implements Pipeline.add(message) for scripts.
func addMethod(self *object.Instance, _ context.Context, message any) object.Object {
	p, err := getPipelineInstance(self)
	if err != nil {
		return err
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return &object.Error{Message: "add() called after complete()"}
	}
	idx := p.nextIndex
	p.nextIndex++
	p.results = append(p.results, nil)
	p.wg.Add(1)
	p.mu.Unlock()

	p.queue <- pipelineItem{index: idx, message: message}
	return &object.Null{}
}

// completeMethod implements Pipeline.complete() for scripts.
func completeMethod(self *object.Instance, ctx context.Context) object.Object {
	p, err := getPipelineInstance(self)
	if err != nil {
		return err
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return &object.Error{Message: "complete() already called"}
	}
	p.closed = true
	p.mu.Unlock()

	close(p.queue)
	object.RunBlocking(ctx, func() { p.wg.Wait() })

	p.mu.Lock()
	results := p.results
	p.mu.Unlock()
	return &object.List{Elements: results}
}

// getPipelineInstance extracts the *PipelineInstance from a scriptling Instance.
func getPipelineInstance(self *object.Instance) (*PipelineInstance, *object.Error) {
	wrapper, ok := object.GetClientField(self, "_pipeline")
	if !ok {
		return nil, &object.Error{Message: "Pipeline: missing internal reference"}
	}
	pi, ok := wrapper.Client.(*PipelineInstance)
	if !ok {
		return nil, &object.Error{Message: "Pipeline: invalid internal reference"}
	}
	return pi, nil
}

// createPipelineInstance wraps a *PipelineInstance in a scriptling object.Instance.
func createPipelineInstance(pi *PipelineInstance) *object.Instance {
	return object.NewInstanceWithFields(GetPipelineClass(), map[string]object.Object{
		"_pipeline": &object.ClientWrapper{
			TypeName: "Pipeline",
			Client:   pi,
		},
	})
}

func buildPipelineClass() *object.Class {
	return object.NewClassBuilder("Pipeline").
		MethodWithHelp("add", addMethod, `add(message) - Add a message to the pipeline

Queues a message for completion. Processing starts immediately as concurrency
slots are available; you do not need to wait until complete() is called.

add() accepts the same message formats as completion() and ask():
  - str: simple user message; the pipeline's system_prompt kwarg (if set)
         is applied automatically
  - list: full conversation as a list of {"role": ..., "content": ...} dicts;
          system_prompt is ignored when a message list is passed

Parameters:
  message (str or list): User message string, or list of message dicts

Returns:
  None

Example:
  # String shorthand
  pipe.add("What is the capital of France?")

  # Full message list
  pipe.add([
      {"role": "system", "content": "You are a geography expert."},
      {"role": "user",   "content": "What is the capital of France?"},
  ])`).
		MethodWithHelp("complete", completeMethod, `complete() - Wait for all queued completions and return results

Closes the pipeline to new additions, waits for all in-flight requests to
finish, and returns results in the same order as the add() calls.

complete() may only be called once. Calling add() after complete() raises an error.

Return value depends on the mode the pipeline was created with:
  ask=False (default, completion mode):
    list of response dicts, same structure as completion().
    Access content with result["choices"][0]["message"]["content"].
  ask=True (ask mode):
    list of plain text strings, same as ask(). Thinking blocks are removed.

Returns:
  list: Ordered results — response dicts (completion mode) or strings (ask mode)

Example:
  # Completion mode
  pipe = client.Pipeline("gpt-4", max_parallel=4)
  pipe.add("What is 2+2?")
  pipe.add("Capital of France?")
  results = pipe.complete()
  for r in results:
      print(r["choices"][0]["message"]["content"])

  # Ask mode
  pipe = client.Pipeline("gpt-4", max_parallel=4, ask=True)
  pipe.add("What is 2+2?")
  answers = pipe.complete()
  print(answers[0])`).
		Build()
}
