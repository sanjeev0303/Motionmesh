package transcode

import (
	"testing"
)

func TestBuildLadderAlwaysReturnsAtLeastOne(t *testing.T) {
	for _, h := range []int{0, 1, 50, 100, 144, 240, 360, 480, 720, 1080, 2160, 4320} {
		ladder := BuildLadder(h)
		if len(ladder) == 0 {
			t.Errorf("BuildLadder(%d) returned empty slice", h)
		}
	}
}

func TestBuildLadderRenditionsDescending(t *testing.T) {
	for _, sourceH := range []int{240, 360, 480, 720, 1080, 2160} {
		ladder := BuildLadder(sourceH)
		for i := 1; i < len(ladder); i++ {
			if ladder[i].Height >= ladder[i-1].Height {
				t.Errorf("BuildLadder(%d): renditions not in descending order at index %d: %d >= %d",
					sourceH, i, ladder[i].Height, ladder[i-1].Height)
			}
		}
	}
}

func TestBuildLadderRenditionFields(t *testing.T) {
	ladder := BuildLadder(1080)
	for _, r := range ladder {
		if r.Label == "" {
			t.Error("rendition has empty label")
		}
		if r.Bitrate == "" {
			t.Error("rendition has empty bitrate")
		}
		if r.Height <= 0 {
			t.Errorf("rendition has invalid height: %d", r.Height)
		}
	}
}

// 4K source should be capped at 1080p (no 4K/2K renditions defined in the ladder).
func TestBuildLadder4KCapsAt1080p(t *testing.T) {
	ladder := BuildLadder(2160)
	for _, r := range ladder {
		if r.Height > 1080 {
			t.Errorf("4K source produced rendition at %dp, expected cap at 1080p", r.Height)
		}
	}
	if len(ladder) < 5 {
		t.Errorf("4K source should produce full 5-rung ladder, got %d", len(ladder))
	}
}

// Full 1080p source gets all 5 rungs.
func TestBuildLadder1080pFullLadder(t *testing.T) {
	ladder := BuildLadder(1080)
	if len(ladder) != 5 {
		t.Errorf("1080p source should produce 5-rung ladder, got %d", len(ladder))
	}
	if ladder[0].Height != 1080 {
		t.Errorf("first rung should be 1080p, got %d", ladder[0].Height)
	}
	if ladder[len(ladder)-1].Height != 240 {
		t.Errorf("last rung should be 240p, got %d", ladder[len(ladder)-1].Height)
	}
}
