package repo

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type Date time.Time

func (d Date) Value() (driver.Value, error) {
	return time.Time(d).Format(time.RFC3339), nil
}

func (d *Date) Scan(value any) error {
	if value == nil {
		*d = Date(time.Time{})
		return nil
	}

	if str, ok := value.(string); ok {
		t, err := time.Parse(time.RFC3339, str)
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05", str)
			if err != nil {
				return err
			}
		}
		*d = Date(t)
		return nil
	}

	if t, ok := value.(time.Time); ok {
		*d = Date(t)
		return nil
	}

	return fmt.Errorf("cannot scan type %T into Date", value)
}

func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(d))
}

func (d *Date) UnmarshalJSON(b []byte) error {
	var t time.Time
	if err := json.Unmarshal(b, &t); err != nil {
		return err
	}
	*d = Date(t)
	return nil
}

func (d Date) String() string {
	return time.Time(d).Format(time.RFC3339)
}

func (d Date) Time() time.Time {
	return time.Time(d)
}
