package review

import "encoding/json"

// MarshalJSON preserves the legacy player_pseudonym field for existing API
// clients while also exposing the same direct DayZ/Steam identity as player_id.
func (candidate Candidate) MarshalJSON() ([]byte, error) {
	type alias Candidate
	return json.Marshal(struct {
		alias
		PlayerID string `json:"player_id"`
	}{
		alias:    alias(candidate),
		PlayerID: candidate.PlayerPseudonym,
	})
}
