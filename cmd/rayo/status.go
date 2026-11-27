package main

import (
	"errors"
	"fmt"

	"github.com/nanihadesuka/rayo/internal/report"
	"github.com/nanihadesuka/rayo/internal/suite"
)

// status says whether each flow passed the last time it ran into the output folder, in whichever run:
// after fixing some and running only those, whether the suite is green without running it all again.
func status(args []string) error {
	o, err := parse(cmdStatus, args)
	if err != nil {
		return err
	}
	if len(o.args) == 0 {
		return errors.New("rayo status takes the flows, a folder or a suite, as rayo test does")
	}
	flows, group, err := suite.Load(o.args)
	if err != nil {
		return err
	}
	o.written(group)
	st := report.State(o.output, flows)
	var red error
	if !st.Green {
		red = exiting(report.ExitFailed, fmt.Errorf("%d of %d flows haven't passed the last time they ran", st.Total-st.Passed, st.Total))
	}
	if printing != report.Human {
		if err := emit(st); err != nil {
			return err
		}
		if red != nil {
			return &exitError{report.ExitFailed, errShown}
		}
		return nil
	}
	fmt.Print(st.Text())
	return red
}
