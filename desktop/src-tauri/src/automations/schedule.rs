use super::types::{AutomationInput, Trigger};
use crate::error::{HostError, HostResult};
use chrono::{
    Datelike, Days, Duration, Local, LocalResult, NaiveDateTime, NaiveTime, Offset, TimeZone,
};

pub fn validate(input: &AutomationInput) -> HostResult<()> {
    if input.title.trim().is_empty()
        || input.title.chars().count() > 160
        || input.prompt.trim().is_empty()
        || input.prompt.chars().count() > 100000
    {
        return Err(invalid());
    }
    match &input.trigger {
        Trigger::Interval { minutes }
            if !minutes.is_finite() || !(1.0..=525600.0).contains(minutes) =>
        {
            return Err(invalid())
        }
        Trigger::Daily { time } | Trigger::Weekly { time, .. } => {
            parse_time(time)?;
        }
        _ => {}
    }
    if let Trigger::Weekly { days, .. } = &input.trigger {
        if days.is_empty() || days.iter().any(|day| *day > 6) {
            return Err(invalid());
        }
    }
    if input
        .trigger
        .cooldown()
        .is_some_and(|minutes| !minutes.is_finite() || minutes < 1.0)
    {
        return Err(invalid());
    }
    Ok(())
}

pub fn next_occurrence(trigger: &Trigger, now: i64) -> HostResult<Option<i64>> {
    next_in_zone(trigger, now, &Local)
}

pub fn next_in_zone<T: TimeZone>(
    trigger: &Trigger,
    now: i64,
    timezone: &T,
) -> HostResult<Option<i64>> {
    if let Trigger::Interval { minutes } = trigger {
        return now
            .checked_add((minutes * 60000.0) as i64)
            .map(Some)
            .ok_or_else(invalid);
    }
    let (time, days) = match trigger {
        Trigger::Daily { time } => (time, None),
        Trigger::Weekly { time, days } => (time, Some(days)),
        _ => return Ok(None),
    };
    let time = parse_time(time)?;
    let local_now = timezone
        .timestamp_millis_opt(now)
        .single()
        .ok_or_else(invalid)?;
    for offset in 0..=7 {
        let date = local_now
            .date_naive()
            .checked_add_days(Days::new(offset))
            .ok_or_else(invalid)?;
        if days.is_some_and(|days| !days.contains(&date.weekday().num_days_from_sunday())) {
            continue;
        }
        let candidate = compatible_local(timezone, date.and_time(time))?;
        if candidate > now {
            return Ok(Some(candidate));
        }
    }
    Err(invalid())
}

fn compatible_local<T: TimeZone>(timezone: &T, value: NaiveDateTime) -> HostResult<i64> {
    match timezone.from_local_datetime(&value) {
        LocalResult::Single(time) | LocalResult::Ambiguous(time, _) => Ok(time.timestamp_millis()),
        LocalResult::None => {
            let mut before = None;
            let mut after = None;
            for minute in 1..=2880 {
                if before.is_none() {
                    before = timezone
                        .from_local_datetime(&(value - Duration::minutes(minute)))
                        .latest()
                        .map(|date| date.offset().fix().local_minus_utc());
                }
                if after.is_none() {
                    after = timezone
                        .from_local_datetime(&(value + Duration::minutes(minute)))
                        .earliest()
                        .map(|date| date.offset().fix().local_minus_utc());
                }
                if let (Some(before), Some(after)) = (before, after) {
                    let adjusted = value + Duration::seconds((after - before) as i64);
                    return timezone
                        .from_local_datetime(&adjusted)
                        .earliest()
                        .map(|date| date.timestamp_millis())
                        .ok_or_else(invalid);
                }
            }
            Err(invalid())
        }
    }
}

fn parse_time(value: &str) -> HostResult<NaiveTime> {
    if value.len() != 5
        || value.as_bytes()[2] != b':'
        || [0, 1, 3, 4]
            .iter()
            .any(|index| !value.as_bytes()[*index].is_ascii_digit())
    {
        return Err(invalid());
    }
    NaiveTime::parse_from_str(value, "%H:%M").map_err(|_| invalid())
}

pub fn invalid() -> HostError {
    HostError::new("AUTOMATION_INVALID", "automation.invalid")
}
