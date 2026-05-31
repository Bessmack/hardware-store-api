package geo

import "math"

// HaversineDistance calculates the great-circle distance in kilometres
// between two lat/lng points on Earth.
// This runs entirely in memory — no API call, no latency, no cost.
func HaversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusKm = 6371

	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180

	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(dLng/2)*math.Sin(dLng/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusKm * c
}

// StoreInfo is the minimal store data needed for distance calculations.
// The stores package populates this; the geo package only works with it.
type StoreInfo struct {
	ID        string
	Name      string
	County    string
	Latitude  float64
	Longitude float64
}

// FindNearestStore returns the store closest to the given coordinates.
// Returns nil if the stores slice is empty.
func FindNearestStore(stores []StoreInfo, lat, lng float64) *StoreInfo {
	if len(stores) == 0 {
		return nil
	}

	nearest := &stores[0]
	minDist := HaversineDistance(lat, lng, stores[0].Latitude, stores[0].Longitude)

	for i := 1; i < len(stores); i++ {
		dist := HaversineDistance(lat, lng, stores[i].Latitude, stores[i].Longitude)
		if dist < minDist {
			minDist = dist
			nearest = &stores[i]
		}
	}

	return nearest
}

// DistanceMetres returns the distance between two coordinate pairs in metres.
// Used by the POD GPS verification layer.
func DistanceMetres(lat1, lng1, lat2, lng2 float64) float64 {
	return HaversineDistance(lat1, lng1, lat2, lng2) * 1000
}