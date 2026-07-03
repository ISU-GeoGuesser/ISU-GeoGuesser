package main

import "math"

// Proximity scoring, matching GeoGuessr's model but scaled to campus size.
//
// GeoGuessr awards score = 5000 * exp(-10 * distance / mapSize), where
// mapSize is the diagonal of the map being played (~14,916 km for the
// world map). Our "map" is the ISU campus, whose playable area is roughly
// 2.5 km corner to corner, so the same formula with campusDiagonalMeters
// produces the same feel: ~5000 up close, ~1840 at a tenth of the map,
// near zero across the whole map.
const (
	maxScore = 5000

	// Approximate diagonal of the playable ISU campus area in meters.
	campusDiagonalMeters = 2500.0

	// Any guess within this radius counts as perfect. GPS coordinates for
	// photos are only accurate to a few meters anyway.
	perfectRadiusMeters = 10.0

	earthRadiusMeters = 6371000.0
)

// haversineMeters returns the great-circle distance between two
// latitude/longitude points in meters.
func haversineMeters(lat1, lng1, lat2, lng2 float64) float64 {
	rad := math.Pi / 180.0
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusMeters * math.Asin(math.Sqrt(a))
}

// scoreForDistance converts a guess error (meters) into round points.
func scoreForDistance(distanceMeters float64) int {
	if distanceMeters <= perfectRadiusMeters {
		return maxScore
	}
	return int(math.Round(maxScore * math.Exp(-10*distanceMeters/campusDiagonalMeters)))
}
