package futures

import (
	"context"

	"github.com/restatedev/sdk-go/internal/wire"
)

type Selectable interface {
	getEntry() (wire.CompleteableMessage, error)
}

type Selector interface {
	Select() bool
	Err() error
	Result() Selectable
}

type selector struct {
	ctx    context.Context
	futs   []Selectable
	chosen Selectable
	err    error
}

func (s *selector) Select() bool {
	if s.err != nil {
		return false
	}
	if len(s.futs) == 0 {
		return false
	}

	// todo pick what element to taike
	i := len(s.futs) - 1

	// pick future
	s.chosen = s.futs[i]
	// swap it to the end
	s.futs[i], s.futs[len(s.futs)-1] = s.futs[len(s.futs)-1], s.futs[i]
	// trim
	s.futs = s.futs[:len(s.futs)-1]

	return true
}

func (s *selector) Result() Selectable {
	// TODO
	return s.chosen
}

func (s *selector) Err() error {
	// TODO
	return s.err
}

func Select(ctx context.Context, futs ...Selectable) Selector {
	s := &selector{ctx: ctx, futs: futs}
	for i := range futs {
		_, err := futs[i].getEntry()
		if err != nil {
			s.err = err
			break
		}
	}
	return s
}
