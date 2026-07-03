package main

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// One degree of latitude is ~111.2 km everywhere on Earth, so 0.001° is a
// handy known distance (~111.2 m) for checking haversine.
func TestHaversineKnownDistance(t *testing.T) {
	quadLat, quadLng := 40.5085, -88.9917

	require.Equal(t, 0.0, haversineMeters(quadLat, quadLng, quadLat, quadLng))

	d := haversineMeters(quadLat, quadLng, quadLat+0.001, quadLng)
	require.InDelta(t, 111.2, d, 0.5)
}

func TestScorePerfectGuess(t *testing.T) {
	require.Equal(t, maxScore, scoreForDistance(0))
	require.Equal(t, maxScore, scoreForDistance(perfectRadiusMeters))
}

func TestScoreMatchesGeoGuessrCurve(t *testing.T) {
	// score = 5000 * e^(-10d / 2500). At d = 250m (a tenth of the campus
	// diagonal) that is 5000/e ≈ 1839, same shape as GeoGuessr's world map.
	require.Equal(t, int(math.Round(5000/math.E)), scoreForDistance(250))

	// Guessing across the whole campus should be worth almost nothing.
	require.Less(t, scoreForDistance(campusDiagonalMeters), 5)
}

func TestScoreDecreasesWithDistance(t *testing.T) {
	prev := scoreForDistance(0)
	for d := 25.0; d <= 3000; d += 25 {
		s := scoreForDistance(d)
		require.LessOrEqual(t, s, prev, "score should never increase with distance (d=%v)", d)
		prev = s
	}
}

func TestPickRoundLocations(t *testing.T) {
	// Fewer rounds than locations: no repeats.
	picked := pickRoundLocations(len(campusLocations) - 1)
	seen := make(map[string]bool)
	for _, loc := range picked {
		require.False(t, seen[loc.Name], "location %q repeated too early", loc.Name)
		seen[loc.Name] = true
	}

	// More rounds than locations still fills the list.
	require.Len(t, pickRoundLocations(len(campusLocations)+3), len(campusLocations)+3)
}
