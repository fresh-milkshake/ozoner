package utils

import (
	"context"
	"sync"
)

// Job represents a job to be processed
type Job[T any] struct {
	ID   string
	Data T
}

// Result represents the result of a job
type Result[T any, R any] struct {
	JobID string
	Data  R
	Error error
}

// WorkerPool manages a pool of workers for concurrent processing
type WorkerPool[T any, R any] struct {
	workerCount int
	jobChan     chan Job[T]
	resultChan  chan Result[T, R]
	wg          sync.WaitGroup
	processor   func(context.Context, T) (R, error)
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool[T any, R any](workerCount int, processor func(context.Context, T) (R, error)) *WorkerPool[T, R] {
	return &WorkerPool[T, R]{
		workerCount: workerCount,
		jobChan:     make(chan Job[T], workerCount*2),
		resultChan:  make(chan Result[T, R], workerCount*2),
		processor:   processor,
	}
}

// Start starts the worker pool
func (wp *WorkerPool[T, R]) Start(ctx context.Context) {
	for i := 0; i < wp.workerCount; i++ {
		wp.wg.Add(1)
		go wp.worker(ctx)
	}
}

// worker processes jobs from the job channel
func (wp *WorkerPool[T, R]) worker(ctx context.Context) {
	defer wp.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-wp.jobChan:
			if !ok {
				return
			}

			result, err := wp.processor(ctx, job.Data)
			wp.resultChan <- Result[T, R]{
				JobID: job.ID,
				Data:  result,
				Error: err,
			}
		}
	}
}

// Submit submits a job to the pool
func (wp *WorkerPool[T, R]) Submit(job Job[T]) {
	wp.jobChan <- job
}

// Results returns the result channel
func (wp *WorkerPool[T, R]) Results() <-chan Result[T, R] {
	return wp.resultChan
}

// Close closes the job channel and waits for workers to finish
func (wp *WorkerPool[T, R]) Close() {
	close(wp.jobChan)
	wp.wg.Wait()
	close(wp.resultChan)
}

