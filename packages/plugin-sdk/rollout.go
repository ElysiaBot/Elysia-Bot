package pluginsdk

type RolloutStatus string

const (
	RolloutStatusPrepared  RolloutStatus = "prepared"
	RolloutStatusRejected  RolloutStatus = "rejected"
	RolloutStatusActivated RolloutStatus = "activated"
)

type RolloutRecord struct {
	PluginID         string        `json:"pluginId"`
	CurrentVersion   string        `json:"currentVersion"`
	CandidateVersion string        `json:"candidateVersion"`
	Status           RolloutStatus `json:"status"`
	Reason           string        `json:"reason,omitempty"`
}
