package main

import "math/rand"

// A campus photo spot the game can use as a round's answer.
//
// HARDCODED for the MVP demo (proposal W3-4 "Hardcoded Locations Demo").
// Once the photo library + admin upload flow lands, this slice gets
// replaced by rows from the locations table.
type gameLocation struct {
	Name  string  `json:"name"`
	Image string  `json:"image"`
	Lat   float64 `json:"lat"`
	Lng   float64 `json:"lng"`
}

// Coordinates are approximate real ISU campus spots (good enough to test
// scoring). The photos are placeholders — every round currently shows the
// same Quad test shot until the real photo set is collected.
var campusLocations = []gameLocation{
	// This first entry is the one real row from the prototype's database.
	{Name: "The Quad, Fell Hall side", Image: "/game/photos/location_1781484201607.jpg", Lat: 40.5085297892111, Lng: -88.9916825294495},
	{Name: "Old Main Bell", Image: "/game/photos/campus-quad.jpg", Lat: 40.50905, Lng: -88.99094},
	{Name: "Milner Library", Image: "/game/photos/location_1781474132768.jpg", Lat: 40.51000, Lng: -88.99323},
	{Name: "Bone Student Center", Image: "/game/photos/location_1781474267550.jpg", Lat: 40.51236, Lng: -88.99017},
	{Name: "Watterson Towers", Image: "/game/photos/location_1781474302483.jpg", Lat: 40.50662, Lng: -88.98831},
	{Name: "Hovey Hall", Image: "/game/photos/location_1781475329357.jpg", Lat: 40.50791, Lng: -88.99383},
	{Name: "Cook Hall", Image: "/game/photos/campus-quad.jpg", Lat: 40.50921, Lng: -88.99138},
	{Name: "Schroeder Hall", Image: "/game/photos/location_1781474132768.jpg", Lat: 40.50846, Lng: -88.99036},
}

// pickRoundLocations returns n locations in random order, only repeating
// once every location has been used.
func pickRoundLocations(n int) []gameLocation {
	picked := make([]gameLocation, 0, n)
	for len(picked) < n {
		for _, i := range rand.Perm(len(campusLocations)) {
			if len(picked) == n {
				break
			}
			picked = append(picked, campusLocations[i])
		}
	}
	return picked
}
