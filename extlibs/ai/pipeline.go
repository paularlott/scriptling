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

// pipelineBackoff represents an active backoff period.  The done channel is
// closed when the backoff expires, waking all waiting workers at once.
type pipelineBackoff struct {
	done chan struct{}
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

	// Adaptive concurrency semaphore (same design as runParallel).
	slots   chan struct{}
	slotsMu sync.Mutex

	backoffMu  sync.Mutex
	curBackoff *pipelineBackoff
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
		slots:      make(chan struct{}, maxParallel),
	}

	for i := 0; i < maxParallel; i++ {
		go p.worker()
	}
	return p
}

// worker drains the queue until it is closed.
func (p *PipelineInstance) worker() {
	for item := range p.queue {
		if !p.acquireSlot() {
			// Context was cancelled; store an error for this slot.
			p.mu.Lock()
			p.results[item.index] = &object.Error{Message: "context cancelled"}
			p.mu.Unlock()
			p.wg.Done()
			continue
		}

		// Always run completion so rate-limit retry metadata is preserved.
		res := completionMethod(p.aiInstance, p.ctx, p.kwargs, p.model, item.message)
		p.releaseSlot()

		// Adaptive rate-limit: halve concurrency and pause workers on 429.
		if respMap, ok := resultAsMap(res); ok {
			if retry, ok := respMap["retry"].(map[string]any); ok {
				if hit, _ := retry["rate_limit_hit"].(bool); hit {
					p.applyRateLimit()
				}
			}
		}

		// For ask mode, convert the completion response to plain text.
		if p.ask {
			if _, isErr := res.(*object.Error); !isErr {
				res = extractTextFromResponse(res)
			}
		}

		p.mu.Lock()
		p.results[item.index] = res
		p.mu.Unlock()
		p.wg.Done()
	}
}

// acquireSlot blocks until a concurrency slot is available or ctx is done.
// Returns false if the context was cancelled.
func (p *PipelineInstance) acquireSlot() bool {
	for {
		// Honour any active backoff before competing for a slot.
		p.backoffMu.Lock()
		bs := p.curBackoff
		p.backoffMu.Unlock()

		if bs != nil {
			select {
			case <-bs.done:
				// backoff elapsed, retry
			case <-p.ctx.Done():
				return false
			}
			continue
		}

		p.slotsMu.Lock()
		s := p.slots
		p.slotsMu.Unlock()

		select {
		case s <- struct{}{}:
			return true
		case <-p.ctx.Done():
			return false
		}
	}
}

// releaseSlot frees one concurrency slot.
func (p *PipelineInstance) releaseSlot() {
	p.slotsMu.Lock()
	s := p.slots
	p.slotsMu.Unlock()
	select {
	case <-s:
	default:
	}
}

// applyRateLimit halves the concurrency limit and imposes a 1-second pause.
func (p *PipelineInstance) applyRateLimit() {
	p.slotsMu.Lock()
	cur := cap(p.slots)
	next := cur / 2
	if next < 1 {
		next = 1
	}
	if next < cur {
		p.slots = make(chan struct{}, next)
	}
	p.slotsMu.Unlock()

	bs := &pipelineBackoff{done: make(chan struct{})}
	p.backoffMu.Lock()
	p.curBackoff = bs
	p.backoffMu.Unlock()

	go func() {
		select {
		case <-time.After(time.Second):
		case <-p.ctx.Done():
		}
		p.backoffMu.Lock()
		if p.curBackoff == bs {
			p.curBackoff = nil
		}
		p.backoffMu.Unlock()
		close(bs.done)
	}()
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
