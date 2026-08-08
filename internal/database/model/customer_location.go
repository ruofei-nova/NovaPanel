package model

// CustomerLocation stores the most recent browser-authorised position for a
// customer. The row is keyed by user so tenant scoping never depends on
// client-supplied identifiers.
type CustomerLocation struct {
	Id          int     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID      int     `json:"userId" gorm:"uniqueIndex;not null"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	AccuracyM   float64 `json:"accuracyM"`
	UpdatedAt   int64   `json:"updatedAt" gorm:"index"`
	IPLatitude  float64 `json:"ipLatitude"`
	IPLongitude float64 `json:"ipLongitude"`
	IPUpdatedAt int64   `json:"ipUpdatedAt" gorm:"index"`
}
