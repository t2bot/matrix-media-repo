package database

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

type AnonymousJson map[string]interface{}

// Value implements driver.Valuer
func (a *AnonymousJson) Value() (driver.Value, error) {
	return json.Marshal(a)
}

// Scan implements sql.Scanner
func (a *AnonymousJson) Scan(value interface{}) error {
	if b, ok := value.([]byte); !ok {
		return errors.New("failed to assert jsonb is bytes")
	} else {
		return json.Unmarshal(b, &a)
	}
}

func (a *AnonymousJson) ApplyTo(val interface{}) error {
	if b, err := json.Marshal(a); err != nil {
		return err
	} else {
		if err = json.Unmarshal(b, &val); err != nil {
			return err
		}
	}
	return nil
}

func (a *AnonymousJson) ApplyFrom(val interface{}) error {
	if b, err := json.Marshal(val); err != nil {
		return err
	} else {
		if err = json.Unmarshal(b, &a); err != nil {
			return err
		}
	}
	return nil
}
