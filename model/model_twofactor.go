package model

// TwoFactor represents a two-factor authentication device.
type TwoFactor struct {
	ID         int
	Account    int
	Timestamp  int64
	LastActive int64
	Secret     string
	Name       string
}
