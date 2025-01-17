package schedule

import (
	"errors"

	"github.com/creativeprojects/resticprofile/config"
)

//
// Job: common code for all systems
//

// SchedulerJob interface
type SchedulerJob interface {
	Accessible() bool
	Create() error
	Remove() error
	RemoveOnly() bool
	Status() error
}

// Job scheduler
type Job struct {
	config  *config.ScheduleConfig
	handler Handler
}

var ErrorJobCanBeRemovedOnly = errors.New("job can be removed only")

// Accessible checks if the current user is permitted to access the job
func (j *Job) Accessible() bool {
	permission, _ := detectSchedulePermission(j.config.Permission)
	return checkPermission(permission)
}

// Create a new job
func (j *Job) Create() error {
	if j.RemoveOnly() {
		return ErrorJobCanBeRemovedOnly
	}

	permission := getSchedulePermission(j.config.Permission)
	ok := checkPermission(permission)
	if !ok {
		return permissionError("create")
	}

	schedules, err := j.handler.ParseSchedules(j.config.Schedules)
	if err != nil {
		return err
	}

	if len(schedules) > 0 {
		j.handler.DisplayParsedSchedules(j.config.SubTitle, schedules)
	} else {
		err := j.handler.DisplaySchedules(j.config.SubTitle, j.config.Schedules)
		if err != nil {
			return err
		}
	}

	err = j.handler.CreateJob(j.config, schedules, permission)
	if err != nil {
		return err
	}

	return nil
}

// Remove a job
func (j *Job) Remove() error {
	var permission string
	if j.RemoveOnly() {
		permission, _ = detectSchedulePermission(j.config.Permission) // silent call for possibly non-existent job
	} else {
		permission = getSchedulePermission(j.config.Permission)
	}
	ok := checkPermission(permission)
	if !ok {
		return permissionError("remove")
	}

	return j.handler.RemoveJob(j.config, permission)
}

// RemoveOnly returns true if this job can be removed only
func (j *Job) RemoveOnly() bool {
	return j.config.RemoveOnly
}

// Status of a job
func (j *Job) Status() error {
	if j.RemoveOnly() {
		return ErrorJobCanBeRemovedOnly
	}

	schedules, err := j.handler.ParseSchedules(j.config.Schedules)
	if err != nil {
		return err
	}

	if len(schedules) > 0 {
		j.handler.DisplayParsedSchedules(j.config.SubTitle, schedules)
	} else {
		err := j.handler.DisplaySchedules(j.config.SubTitle, j.config.Schedules)
		if err != nil {
			return err
		}
	}

	err = j.handler.DisplayJobStatus(j.config)
	if err != nil {
		return err
	}
	return nil
}

// Verify interface
var _ SchedulerJob = &Job{}
