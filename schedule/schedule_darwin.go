//+build darwin

// Documentation about launchd plist file format:
// https://www.launchd.info

package schedule

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/creativeprojects/resticprofile/calendar"
	"howett.net/plist"
)

// Default paths for launchd files
const (
	launchdBin      = "launchd"
	launchctlBin    = "launchctl"
	UserAgentPath   = "Library/LaunchAgents"
	GlobalAgentPath = "/Library/LaunchAgents"
	GlobalDaemons   = "/Library/LaunchDaemons"

	namePrefix     = "local.resticprofile."
	agentExtension = ".agent.plist"
)

// LaunchJob is an agent definition for launchd
type LaunchJob struct {
	Label                 string             `plist:"Label"`
	Program               string             `plist:"Program"`
	ProgramArguments      []string           `plist:"ProgramArguments"`
	EnvironmentVariables  map[string]string  `plist:"EnvironmentVariables,omitempty"`
	StandardInPath        string             `plist:"StandardInPath,omitempty"`
	StandardOutPath       string             `plist:"StandardOutPath,omitempty"`
	StandardErrorPath     string             `plist:"StandardErrorPath,omitempty"`
	WorkingDirectory      string             `plist:"WorkingDirectory"`
	StartInterval         int                `plist:"StartInterval,omitempty"`
	StartCalendarInterval []CalendarInterval `plist:"StartCalendarInterval,omitempty"`
}

// CalendarInterval contains date and time trigger definition
type CalendarInterval struct {
	Month   int `plist:"Month,omitempty"`   // Month of year (1..12, 1 being January)
	Day     int `plist:"Day,omitempty"`     // Day of month (1..31)
	Weekday int `plist:"Weekday,omitempty"` // Day of week (0..7, 0 and 7 being Sunday)
	Hour    int `plist:"Hour,omitempty"`    // Hour of day (0..23)
	Minute  int `plist:"Minute,omitempty"`  // Minute of hour (0..59)
}

// createJob creates a plist file and register it with launchd
func (j *Job) createJob() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	binary := absolutePathToBinary(wd, os.Args[0])

	name := getJobName(j.profile.Name)
	job := &LaunchJob{
		Label:   name,
		Program: binary,
		ProgramArguments: []string{
			binary,
			"--no-ansi",
			"--config",
			j.configFile,
			"--name",
			j.profile.Name,
			"backup",
		},
		EnvironmentVariables:  j.profile.Environment,
		StandardOutPath:       name + ".log",
		StandardErrorPath:     name + ".log",
		WorkingDirectory:      wd,
		StartCalendarInterval: getCalendarIntervalsFromSchedules(j.schedules),
	}

	file, err := os.Create(path.Join(home, UserAgentPath, name+agentExtension))
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := plist.NewEncoder(file)
	encoder.Indent("\t")
	err = encoder.Encode(job)
	if err != nil {
		return err
	}

	// // load the service
	// filename := path.Join(home, UserAgentPath, name+agentExtension)
	// cmd := exec.Command(launchctlBin, "load", filename)
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr
	// err = cmd.Run()
	// if err != nil {
	// 	return err
	// }

	// // start the service
	// cmd = exec.Command(launchctlBin, "start", name)
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr
	// err = cmd.Run()
	// if err != nil {
	// 	return err
	// }

	return nil
}

// RemoveJob stops and unloads the agent from launchd, then removes the configuration file
func RemoveJob(profileName string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	name := getJobName(profileName)

	// stop the service
	stop := exec.Command(launchctlBin, "stop", name)
	stop.Stdout = os.Stdout
	stop.Stderr = os.Stderr
	// keep going if there's an error here
	_ = stop.Run()

	// unload the service
	filename := path.Join(home, UserAgentPath, name+agentExtension)
	unload := exec.Command(launchctlBin, "unload", filename)
	unload.Stdout = os.Stdout
	unload.Stderr = os.Stderr
	err = unload.Run()
	if err != nil {
		return err
	}

	return os.Remove(filename)
}

// checkSystem verifies launchd is available on this system
func checkSystem() error {
	found, err := exec.LookPath(launchdBin)
	if err != nil || found == "" {
		return errors.New("it doesn't look like launchd is installed on your system")
	}
	return nil
}

func (j *Job) displayStatus() error {
	cmd := exec.Command(launchctlBin, "list", getJobName(j.profile.Name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}

func getJobName(profileName string) string {
	return namePrefix + strings.ToLower(profileName)
}

func loadSchedules(schedules []string) ([]*calendar.Event, error) {
	now := time.Now().Round(time.Second)
	events := make([]*calendar.Event, 0, len(schedules))
	for index, schedule := range schedules {
		if schedule == "" {
			return events, errors.New("empty schedule")
		}
		fmt.Printf("\nAnalyzing schedule %d/%d\n========================\n", index+1, len(schedules))
		event := calendar.NewEvent()
		err := event.Parse(schedule)
		if err != nil {
			return events, err
		}
		next := event.Next(now)
		fmt.Printf("  Original form: %s\n", schedule)
		fmt.Printf("Normalized form: %s\n", event.String())
		fmt.Printf("    Next elapse: %s\n", next.Format(time.UnixDate))
		fmt.Printf("       (in UTC): %s\n", next.UTC().Format(time.UnixDate))
		fmt.Printf("       From now: %s left\n", next.Sub(now))
		events = append(events, event)
	}
	fmt.Print("\n")
	return events, nil
}

// getCalendarIntervalsFromSchedules converts schedules into launchd calendar events
// let's say we've setup these rules:
// Mon-Fri *-*-* *:0,30:00  = every half hour
// Sat     *-*-* 0,12:00:00 = twice a day on saturday
//         *-*-01 *:*:*     = the first of each month
//
// it should translate as:
// 1st rule
//    Weekday = Monday, Minute = 0
//    Weekday = Monday, Minute = 30
//    ... same from Tuesday to Thurday
//    Weekday = Friday, Minute = 0
//    Weekday = Friday, Minute = 30
// Total of 10 rules
// 2nd rule
//    Weekday = Saturday, Hour = 0
//    Weekday = Saturday, Hour = 12
// Total of 2 rules
// 3rd rule
//    Day = 1
// Total of 1 rule
func getCalendarIntervalsFromSchedules(schedules []*calendar.Event) []CalendarInterval {
	entries := make([]CalendarInterval, 0, len(schedules))
	for _, schedule := range schedules {
		entries = append(entries, getCalendarIntervalsFromSchedule(schedule)...)
	}
	return entries
}

func getCalendarIntervalsFromSchedule(schedule *calendar.Event) []CalendarInterval {
	// how many entries will I need?
	total := 0
	values := []*calendar.Value{
		schedule.WeekDay,
		schedule.Month,
		schedule.Day,
		schedule.Hour,
		schedule.Minute,
	}
	for _, value := range values {
		if value.HasValue() {
			num := len(value.GetRangeValues())
			total = permutations(total, num)
		}
	}
	entries := make([]CalendarInterval, total)
	// now go through them all again and fill in the events
	permutation := 1

	// can't do anything generic here...
	if schedule.WeekDay.HasValue() {
		values := schedule.WeekDay.GetRangeValues()
		line := spread(values, total, &permutation)
		for i := 0; i < total; i++ {
			entries[i].Weekday = line[i]
		}
	}

	return entries
}

func permutations(total, num int) int {
	if total == 0 {
		return num
	}
	return total * num
}
