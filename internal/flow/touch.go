package flow

import (
	"errors"
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
	args := agent.Args{"ms": argOr(c, keyDuration, 250*time.Millisecond).Milliseconds()}
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

func scrollUntil(c *call) error {
	d := argOr(c, keyDirection, Direction{0, 1})
	sel := arg(c, keyElement)
	for deadline := time.Now().Add(c.limit); ; {
		_, err := c.r.agent.Do(c.ctx, "wait", agent.Args{"sel": sel}, 100*time.Millisecond)
		if err == nil || c.r.agent.Dead() != nil || time.Now().After(deadline) {
			return err
		}
		// Scrolling down is a swipe up.
		if _, err := c.r.agent.Do(c.ctx, "swipe", agent.Args{"x": 50 + d.x*20, "y": 50 + d.y*20, "x2": 50 - d.x*20, "y2": 50 - d.y*20,
			"percent": true, "ms": 250}, time.Second); err != nil {
			return err
		}
	}
}
