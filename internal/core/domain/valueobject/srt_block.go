package valueobject

// SRTBlock, bir altyazı bloğunu temsil eden domain value object'idir.
type SRTBlock struct {
	Index     int
	Timestamp string
	Text      string
}
