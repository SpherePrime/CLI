package customagents

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type CronSchedule struct {
	minutes     [60]bool
	hours       [24]bool
	daysOfMonth [32]bool
	months      [13]bool
	daysOfWeek  [7]bool
	domWildcard bool
	dowWildcard bool
}

func ParseCron(expr string) (CronSchedule, error) {
	var sched CronSchedule
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return sched, fmt.Errorf("cron expression must have 5 fields, got %d", len(fields))
	}

	minMask, _, err := parseCronField(fields[0], 0, 59, false)
	if err != nil {
		return sched, fmt.Errorf("invalid minute field %q: %w", fields[0], err)
	}
	copy(sched.minutes[:], minMask)

	hourMask, _, err := parseCronField(fields[1], 0, 23, false)
	if err != nil {
		return sched, fmt.Errorf("invalid hour field %q: %w", fields[1], err)
	}
	copy(sched.hours[:], hourMask)

	domMask, domWild, err := parseCronField(fields[2], 1, 31, false)
	if err != nil {
		return sched, fmt.Errorf("invalid day-of-month field %q: %w", fields[2], err)
	}
	for i := 1; i <= 31; i++ {
		sched.daysOfMonth[i] = domMask[i-1]
	}
	sched.domWildcard = domWild

	monthMask, _, err := parseCronField(fields[3], 1, 12, false)
	if err != nil {
		return sched, fmt.Errorf("invalid month field %q: %w", fields[3], err)
	}
	for i := 1; i <= 12; i++ {
		sched.months[i] = monthMask[i-1]
	}

	dowMask, dowWild, err := parseCronField(fields[4], 0, 7, true)
	if err != nil {
		return sched, fmt.Errorf("invalid day-of-week field %q: %w", fields[4], err)
	}
	for i := 0; i < 7; i++ {
		sched.daysOfWeek[i] = dowMask[i]
	}
	if len(dowMask) > 7 && dowMask[7] {
		sched.daysOfWeek[0] = true
	}
	sched.dowWildcard = dowWild

	return sched, nil
}

func parseCronField(field string, minVal, maxVal int, isDOW bool) ([]bool, bool, error) {
	size := maxVal - minVal + 1
	mask := make([]bool, size)
	trimmed := strings.TrimSpace(field)
	if trimmed == "" {
		return nil, false, fmt.Errorf("empty field")
	}
	wildcard := trimmed == "*"

	parts := strings.Split(trimmed, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, false, fmt.Errorf("empty list item")
		}
		step := 1
		rangePart := part
		if idx := strings.IndexByte(part, '/'); idx >= 0 {
			rangePart = part[:idx]
			stepStr := part[idx+1:]
			parsedStep, err := strconv.Atoi(stepStr)
			if err != nil || parsedStep <= 0 {
				return nil, false, fmt.Errorf("invalid step %q", stepStr)
			}
			step = parsedStep
		}

		var start, end int
		if rangePart == "*" {
			start = minVal
			end = maxVal
			if isDOW {
				end = 6
			}
		} else if dash := strings.IndexByte(rangePart, '-'); dash >= 0 {
			sStr := rangePart[:dash]
			eStr := rangePart[dash+1:]
			var err error
			start, err = strconv.Atoi(sStr)
			if err != nil {
				return nil, false, fmt.Errorf("invalid range start %q", sStr)
			}
			end, err = strconv.Atoi(eStr)
			if err != nil {
				return nil, false, fmt.Errorf("invalid range end %q", eStr)
			}
			if start < minVal || end > maxVal || start > end {
				return nil, false, fmt.Errorf("range %d-%d out of bounds [%d,%d]", start, end, minVal, maxVal)
			}
		} else {
			val, err := strconv.Atoi(rangePart)
			if err != nil {
				return nil, false, fmt.Errorf("invalid value %q", rangePart)
			}
			if val < minVal || val > maxVal {
				return nil, false, fmt.Errorf("value %d out of bounds [%d,%d]", val, minVal, maxVal)
			}
			start = val
			if strings.Contains(part, "/") {
				end = maxVal
				if isDOW {
					end = 6
				}
			} else {
				end = val
			}
		}

		for v := start; v <= end; v += step {
			mask[v-minVal] = true
		}
	}

	return mask, wildcard, nil
}

func (s CronSchedule) matchesDay(t time.Time) bool {
	domMatch := s.daysOfMonth[t.Day()]
	dowMatch := s.daysOfWeek[int(t.Weekday())]
	if !s.domWildcard && !s.dowWildcard {
		return domMatch || dowMatch
	}
	if !s.domWildcard {
		return domMatch
	}
	if !s.dowWildcard {
		return dowMatch
	}
	return true
}

func (s CronSchedule) NextAfter(t time.Time) time.Time {
	loc := t.Location()
	cur := t.Truncate(time.Minute).Add(time.Minute)
	limit := cur.AddDate(4, 0, 0)

	for cur.Before(limit) {
		if !s.months[int(cur.Month())] {
			cur = time.Date(cur.Year(), cur.Month()+1, 1, 0, 0, 0, 0, loc)
			continue
		}
		if !s.matchesDay(cur) {
			cur = time.Date(cur.Year(), cur.Month(), cur.Day()+1, 0, 0, 0, 0, loc)
			continue
		}
		if !s.hours[cur.Hour()] {
			cur = time.Date(cur.Year(), cur.Month(), cur.Day(), cur.Hour()+1, 0, 0, 0, loc)
			continue
		}
		if !s.minutes[cur.Minute()] {
			cur = cur.Add(time.Minute)
			continue
		}
		return cur
	}
	return time.Time{}
}
