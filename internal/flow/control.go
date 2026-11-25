package flow

import (
	"context"
	"errors"
	"maps"
	"math"
	"slices"
	"time"

	"github.com/nanihadesuka/rayo/internal/agent"
	"gopkg.in/yaml.v3"
)

// The commands made of other commands.

func runFlow(c *call) error {
	if when := arg(c, keyWhen); when != nil {
		ok, err := c.r.holds(c.ctx, when)
		if err != nil {
			return err
		}
		if !ok {
			c.show(Skipped)
			return nil
		}
	}
	return c.group(func(steps []*Step) error { return c.inside(steps) })
}

func retry(c *call) error {
	return c.group(func(steps []*Step) error {
		times := argOr(c, keyRetries, 1)
		for i := 0; ; i++ {
			err := c.inside(steps)
			if err == nil || errors.Is(err, errUntil) || i >= times || c.r.agent.Dead() != nil || c.ctx.Err() != nil {
				return err
			}
		}
	})
}

func repeat(c *call) error {
	return c.group(func(steps []*Step) error {
		for i := argOr(c, keyTimes, math.MaxInt); i > 0; i-- {
			if err := c.ctx.Err(); err != nil {
				return err
			}
			if while := arg(c, keyWhile); while != nil {
				if ok, err := c.r.holds(c.ctx, while); err != nil || !ok {
					return err
				}
			}
			if err := c.inside(steps); err != nil {
				return err
			}
		}
		return nil
	})
}

func (c *call) show(status Status) {
	if status == Skipped {
		c.r.skipped = true
	}
	c.r.reporter.Step(StepReport{Depth: c.depth, Status: status, Text: c.s.describe()})
}

func (c *call) inside(steps []*Step) error { return c.r.steps(c.ctx, steps, c.depth+1) }

// group runs the commands written here, or those of another flow, with their variables.
func (c *call) group(run func(steps []*Step) error) error {
	c.show(Started)
	env := maps.Clone(keyEnv.of(c.v))
	if env == nil {
		env = map[string]string{}
	}
	steps := c.s.Sub
	if file := keyFile.of(c.v); file != "" {
		f, err := Load(relative(c.s.Path, file))
		if err != nil {
			return err
		}
		if err := c.r.loadMocks(f); err != nil {
			return err
		}
		steps = slices.Concat(f.OnStart, f.Steps, f.OnComplete)
		// The file's header is a default: what the caller, its env or -e set wins.
		return c.r.script.with(env, func() error {
			defaults, err := c.r.script.header(f.Env)
			if err != nil {
				return err
			}
			return c.r.script.with(defaults, func() error { return run(steps) })
		})
	}
	return c.r.script.with(env, func() error { return run(steps) })
}

// holds says whether every condition does.
func (r *Runner) holds(ctx context.Context, when *yaml.Node) (bool, error) {
	if n := get(when, keyPlatform); n != nil {
		n, err := r.script.resolve(n)
		if err == nil {
			err = filledIn(n, keyPlatform.check)
		}
		if err != nil {
			return false, err
		}
		if here, err := keyPlatform.kind.read(n); err != nil || !here {
			return false, err
		}
	}
	if n := get(when, keyTrue); n != nil {
		if ok, err := r.script.truthy(n.Value); err != nil || !ok {
			return false, err
		}
	}
	limit := r.scaled(peekTimeout)
	if n := get(when, keyPeek); n != nil {
		n, err := r.script.resolve(n)
		if err == nil {
			err = filledIn(n, keyPeek.check)
		}
		if err != nil {
			return false, err
		}
		limit = keyPeek.value(n).(time.Duration)
	}
	for _, c := range []struct {
		key *Key[agent.Args]
		op  string
	}{{keyVisible, "wait"}, {keyNotVisible, "gone"}} {
		n := get(when, c.key)
		if n == nil {
			continue
		}
		n, err := r.script.resolve(n)
		if err == nil {
			err = filledIn(n, (*reader).selector)
		}
		if err != nil {
			return false, err
		}
		if _, err := r.agent.Do(ctx, c.op, agent.Args{"sel": selector(n)}, limit); err != nil {
			return false, r.agent.Dead() // otherwise it just doesn't hold
		}
	}
	return true, nil
}
