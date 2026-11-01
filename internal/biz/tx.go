package biz

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// Transactor runs a function inside a database transaction. Implemented by the
// data layer; domain code never imports GORM.
type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Step is one unit of work in a saga. Undo must be idempotent because
// compensation may be retried.
type Step struct {
	Name string
	Do   func(ctx context.Context) error
	Undo func(ctx context.Context) error
}

// Saga coordinates steps that may span services.
type Saga interface {
	Run(ctx context.Context, steps []Step) error
}

// TxSaga executes every step inside a single database transaction. It is the
// default in the modular monolith, where product and order share one database.
type TxSaga struct {
	Tx  Transactor
	Log *log.Helper
}

func (s *TxSaga) Run(ctx context.Context, steps []Step) error {
	if s.Tx == nil {
		return errors.New("tx saga: transactor is nil")
	}
	return s.Tx.InTx(ctx, func(txCtx context.Context) error {
		for _, step := range steps {
			if err := step.Do(txCtx); err != nil {
				if s.Log != nil {
					s.Log.Errorf("saga step %q failed, transaction will roll back: %v", step.Name, err)
				}
				return fmt.Errorf("step %s: %w", step.Name, err)
			}
		}
		return nil
	})
}

// CompensationSaga executes steps sequentially and runs the Undo functions of
// completed steps in reverse order when a step fails. It is used when steps
// cross process boundaries (for example order -> product gRPC) where a single
// database transaction is impossible. It does not require DTM.
type CompensationSaga struct {
	Log *log.Helper
}

func (s *CompensationSaga) Run(ctx context.Context, steps []Step) error {
	completed := make([]Step, 0, len(steps))
	for _, step := range steps {
		if err := step.Do(ctx); err != nil {
			original := fmt.Errorf("step %s: %w", step.Name, err)
			if s.Log != nil {
				s.Log.Errorf("saga step %q failed, compensating %d step(s): %v", step.Name, len(completed), err)
			}
			return errors.Join(original, s.compensate(ctx, completed))
		}
		completed = append(completed, step)
	}
	return nil
}

func (s *CompensationSaga) compensate(ctx context.Context, completed []Step) error {
	var errs []error
	for i := len(completed) - 1; i >= 0; i-- {
		step := completed[i]
		if step.Undo == nil {
			continue
		}
		if err := step.Undo(ctx); err != nil {
			if s.Log != nil {
				s.Log.Errorf("compensation for %q failed: %v", step.Name, err)
			}
			errs = append(errs, fmt.Errorf("compensate %s: %w", step.Name, err))
		}
	}
	return errors.Join(errs...)
}

// Clock makes time-dependent behavior testable.
type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }
