package flow

import (
	"errors"
	"fmt"
	"time"

	"github.com/nanihadesuka/rayo/internal/agent"
	"gopkg.in/yaml.v3"
)

// selector turns a flow's form into the agent's.
func selector(n *yaml.Node) agent.Args {
	if n.Kind == yaml.ScalarNode {
		return agent.Args{"text": n.Value}
	}
	sel := agent.Args{}
	_ = pairs(n, func(name, v *yaml.Node) error {
		if k := find(selectorKeys, name.Value); k != nil {
			sel[name.Value] = k.value(v)
		}
		return nil
	})
	return sel
}

// tap fills in where; args already says how.
func tap(c *call, args agent.Args) error {
	sel := selector(c.v)
	if get(c.v, keyPoint) == nil {
		args["sel"] = sel
		return c.do("tap", args)
	}
	at := arg(c, keyPoint)
	if len(sel) > 0 { // a point of the element
		if !at.percent {
			return errors.New("a point of an element is in %")
		}
		args["sel"], args["fx"], args["fy"] = sel, at.x/100, at.y/100
		return c.do("tap", args)
	}
	args["x"], args["y"], args["percent"] = at.x, at.y, at.percent
	return c.do("tapAt", args)
}

func swipe(c *call) error {
	args := agent.Args{"ms": argOr(c, keyDuration, 250*time.Millisecond).Milliseconds(), "hold": !argOr(c, keyFling, true)}
	if get(c.v, keyDirection) != nil {
		d := arg(c, keyDirection)
		if from := arg(c, keyFrom); from != nil {
			args["sel"], args["dx"], args["dy"] = from, d.x*0.5, d.y*0.5
			return c.do("swipeOn", args)
		}
		args["x"], args["y"], args["x2"], args["y2"] = 50-d.x*30, 50-d.y*25, 50+d.x*30, 50+d.y*25
		args["percent"] = true
		return c.do("swipe", args)
	}
	from, to := arg(c, keyStart), arg(c, keyEnd)
	if from.percent != to.percent {
		return errors.New("start and end must both be in % or both in pixels")
	}
	args["x"], args["y"], args["x2"], args["y2"], args["percent"] = from.x, from.y, to.x, to.y, from.percent
	return c.do("swipe", args)
}

// scrollUntil stops once the element is whole on screen, or the list can go no further with some of it
// showing. It drags rather than flings, so the list moves just as far as the finger and never past it (#39).
func scrollUntil(c *call) error {
	d := argOr(c, keyDirection, Direction{0, 1})
	sel := arg(c, keyElement)
	center := argOr(c, keyCenter, false)
	ms := (100-argOr(c, keyScrollSpeed, 40))*10 + 1 // as Maestro's, also the default
	last, same, seen := "", 0, false
	for deadline, scrolls := time.Now().Add(c.limit), 0; ; scrolls++ {
		a, err := c.r.agent.Do(c.ctx, "wait", agent.Args{"sel": sel, "across": d.y == 0}, 100*time.Millisecond)
		// Scrolling down moves the element up: its middle reaches the list's from below.
		there := err == nil && a.Whole && (!center || a.Past*float64(d.x+d.y) <= 0.05)
		if there || c.r.agent.Dead() != nil || time.Now().After(deadline) {
			return err
		}
		seen = err == nil
		// A swipe that changed nothing, twice in a row, is the end of the list: going on would only wait.
		if t, e := c.r.agent.Do(c.ctx, "tree", nil, 5*time.Second); e == nil {
			if scrolls > 0 && t.Out == last {
				if same++; same >= 2 {
					if seen {
						return nil // as much of it as there will ever be
					}
					return fmt.Errorf("%w (the end of the list after %d scrolls: the screen stopped changing)", err, scrolls)
				}
			} else {
				same = 0
			}
			last = t.Out
		}
		// Scrolling down is a swipe up. Once some of it shows, a shorter one brings the rest.
		step := 20.0
		if seen {
			step = 8
		}
		if _, err := c.r.agent.Do(c.ctx, "swipe", agent.Args{"x": 50 + d.x*step, "y": 50 + d.y*step, "x2": 50 - d.x*step, "y2": 50 - d.y*step,
			"percent": true, "ms": ms, "hold": true}, 5*time.Second); err != nil {
			return err
		}
	}
}
