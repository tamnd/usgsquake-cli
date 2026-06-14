package usgsquake

// Earthquake is one seismic event from the USGS catalog.
type Earthquake struct {
	ID           string  `json:"id"`
	Place        string  `json:"place"`
	Title        string  `json:"title"`
	Status       string  `json:"status"`
	Type         string  `json:"type"`
	Magnitude    float64 `json:"magnitude"`
	Time         int64   `json:"time"`     // Unix milliseconds
	Updated      int64   `json:"updated"`  // Unix milliseconds
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	Depth        float64 `json:"depth_km"`
	Tsunami      int     `json:"tsunami"`
	Significance int     `json:"significance"`
	URL          string  `json:"url"`
}

// Count is the result of a /count query.
type Count struct {
	Count int `json:"count"`
}
