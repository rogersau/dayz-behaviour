package audio_test

import (
	"testing"

	"github.com/rogersau/dayz-behaviour/internal/audio"
)

func TestGunshotClassificationAccountsForSuppressor(t *testing.T) {
	if got := audio.ClassifyGunshot(audio.GunshotInput{DistanceM: 250}); got.Audibility != audio.Strong {
		t.Fatalf("unsuppressed 250m = %s, want %s", got.Audibility, audio.Strong)
	}
	if got := audio.ClassifyGunshot(audio.GunshotInput{DistanceM: 250, Suppressed: true}); got.Audibility != audio.Possible {
		t.Fatalf("suppressed 250m = %s, want %s", got.Audibility, audio.Possible)
	}
	if got := audio.ClassifyGunshot(audio.GunshotInput{DistanceM: 500, Suppressed: true}); got.Audibility != audio.NotAudible {
		t.Fatalf("suppressed 500m = %s, want %s", got.Audibility, audio.NotAudible)
	}
}

func TestFootstepClassificationUsesMovementAndContext(t *testing.T) {
	loud := audio.ClassifyFootstep(audio.FootstepInput{
		DistanceM: 18, SpeedMPS: 5, Stance: "ERECT", SurfaceType: "cp_concrete", Footwear: "CombatBoots_Black",
	})
	if loud.Audibility != audio.Likely {
		t.Fatalf("sprinting boots on concrete = %s, want %s", loud.Audibility, audio.Likely)
	}

	quiet := audio.ClassifyFootstep(audio.FootstepInput{
		DistanceM: 18, SpeedMPS: 1, Stance: "CROUCH", SurfaceType: "cp_grass", Footwear: "",
	})
	if quiet.Audibility != audio.NotAudible {
		t.Fatalf("crouch barefoot on grass = %s, want %s", quiet.Audibility, audio.NotAudible)
	}
}
