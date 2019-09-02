package types

type BackgroundTask struct {
	ID      int
	Name    string
	Params  map[string]interface{}
	StartTs int64
	EndTs   int64
}
