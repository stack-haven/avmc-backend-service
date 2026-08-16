package data

import "time"

// setOptionalString assigns value to *target only when value is non-empty.
// Used by async_task_repo for nil-safe Proto message field population.
func setOptionalString(target **string, value string) {
	if value != "" {
		v := value
		*target = &v
	}
}

// setOptionalTime assigns a formatted time to *target only when value is non-nil.
// Used by async_task_repo for nil-safe Proto message field population.
func setOptionalTime(target **string, value *time.Time) {
	if value != nil && !value.IsZero() {
		formatted := value.Format(time.DateTime)
		*target = &formatted
	}
}
