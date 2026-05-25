package main

import (
	"embed"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"html"
	"html/template"
	"io"
	"io/fs"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // embed IANA tz database so LoadLocation works on Windows without external tz data
)

type Config struct {
	Port           int     `json:"port"`
	N2YOKey        string  `json:"n2yo_key,omitempty"`
	PrimaryLabel   string  `json:"primary_label"`
	PrimaryLat     float64 `json:"primary_lat"`
	PrimaryLon     float64 `json:"primary_lon"`
	PrimaryTZ      string  `json:"primary_tz"`
	SecondaryLabel string  `json:"secondary_label"`
	SecondaryTZ    string  `json:"secondary_tz"`
}

func defaultConfig() Config {
	return Config{
		Port:           7777,
		PrimaryLabel:   "Hamilton",
		PrimaryLat:     43.2557,
		PrimaryLon:     -79.8711,
		PrimaryTZ:      "America/Toronto",
		SecondaryLabel: "Adelaide",
		SecondaryTZ:    "Australia/Adelaide",
	}
}

func loadConfig(path string) Config {
	c := defaultConfig()
	b, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	_ = json.Unmarshal(b, &c)
	// Backfill any missing fields from defaults.
	d := defaultConfig()
	if c.Port == 0 {
		c.Port = d.Port
	}
	if c.PrimaryLabel == "" {
		c.PrimaryLabel = d.PrimaryLabel
	}
	if c.PrimaryLat == 0 && c.PrimaryLon == 0 {
		c.PrimaryLat = d.PrimaryLat
		c.PrimaryLon = d.PrimaryLon
	}
	if c.PrimaryTZ == "" {
		c.PrimaryTZ = d.PrimaryTZ
	}
	if c.SecondaryLabel == "" {
		c.SecondaryLabel = d.SecondaryLabel
	}
	if c.SecondaryTZ == "" {
		c.SecondaryTZ = d.SecondaryTZ
	}
	return c
}

func saveConfig(path string, c Config) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

//go:embed web
var webFS embed.FS

const (
	userAgent = "xeneon-dash/0.1 (local-only personal dashboard)"
)

type sourceState struct {
	mu          sync.Mutex
	name        string
	ttl         time.Duration
	fetch       func() (any, error)
	payload     any
	lastSuccess time.Time
	lastError   string
	failures    int
	refreshing  bool
}

type cacheReply struct {
	Payload any
	AgeSec  int
	Status  string
	Error   string
}

func newCache(name string, ttl time.Duration, fetch func() (any, error)) *sourceState {
	return &sourceState{name: name, ttl: ttl, fetch: fetch}
}

func (s *sourceState) get() cacheReply {
	s.mu.Lock()
	age := time.Since(s.lastSuccess)
	has := s.payload != nil
	fresh := has && age < s.ttl
	s.mu.Unlock()

	if !has {
		start := time.Now()
		v, err := s.safeFetch()
		s.mu.Lock()
		defer s.mu.Unlock()
		if err != nil {
			s.lastError = err.Error()
			s.failures++
			log.Printf("[%s] FAIL in %s: %v", s.name, time.Since(start).Truncate(time.Millisecond), err)
			return cacheReply{Status: "dead", Error: s.lastError}
		}
		s.payload = v
		s.lastSuccess = time.Now()
		s.lastError = ""
		s.failures = 0
		log.Printf("[%s] refreshed in %s", s.name, time.Since(start).Truncate(time.Millisecond))
		return cacheReply{Payload: v, AgeSec: 0, Status: "live"}
	}

	if !fresh {
		s.kickRefresh()
	}

	return cacheReply{
		Payload: s.payload,
		AgeSec:  int(age.Seconds()),
		Status:  s.statusForAge(age),
		Error:   s.lastError,
	}
}

func (s *sourceState) statusForAge(age time.Duration) string {
	switch {
	case age < 2*s.ttl:
		return "live"
	case age < 5*s.ttl:
		return "stale"
	default:
		return "dead"
	}
}

func (s *sourceState) kickRefresh() {
	s.mu.Lock()
	if s.refreshing {
		s.mu.Unlock()
		return
	}
	s.refreshing = true
	s.mu.Unlock()

	go func() {
		start := time.Now()
		v, err := s.safeFetch()
		s.mu.Lock()
		defer s.mu.Unlock()
		s.refreshing = false
		if err != nil {
			s.lastError = err.Error()
			s.failures++
			log.Printf("[%s] FAIL in %s: %v", s.name, time.Since(start).Truncate(time.Millisecond), err)
			return
		}
		s.payload = v
		s.lastSuccess = time.Now()
		s.lastError = ""
		s.failures = 0
		log.Printf("[%s] refreshed in %s", s.name, time.Since(start).Truncate(time.Millisecond))
	}()
}

func (s *sourceState) safeFetch() (v any, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("fetch panic: %v", r)
		}
	}()
	return s.fetch()
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

func httpGetJSON(url string, target any) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func httpGetXML(url string, target any) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	return xml.NewDecoder(resp.Body).Decode(target)
}

type openMeteoResp struct {
	Current struct {
		Temp          float64 `json:"temperature_2m"`
		ApparentTemp  float64 `json:"apparent_temperature"`
		WeatherCode   int     `json:"weather_code"`
		WindSpeed     float64 `json:"wind_speed_10m"`
		WindDirection float64 `json:"wind_direction_10m"`
	} `json:"current"`
	Hourly struct {
		Time        []string  `json:"time"`
		Temperature []float64 `json:"temperature_2m"`
		PrecipProb  []int     `json:"precipitation_probability"`
		WeatherCode []int     `json:"weather_code"`
	} `json:"hourly"`
	Daily struct {
		Time          []string  `json:"time"`
		TempMax       []float64 `json:"temperature_2m_max"`
		TempMin       []float64 `json:"temperature_2m_min"`
		WeatherCode   []int     `json:"weather_code"`
		PrecipProbMax []int     `json:"precipitation_probability_max"`
	} `json:"daily"`
}

type weatherOut struct {
	Current  weatherCurrent   `json:"current"`
	Hourly   []weatherHour    `json:"hourly"`
	Summary  string           `json:"summary"`
	Tomorrow *weatherTomorrow `json:"tomorrow,omitempty"`
}

type weatherTomorrow struct {
	HighC        float64 `json:"high_c"`
	LowC         float64 `json:"low_c"`
	Description  string  `json:"description"`
	PrecipPctMax int     `json:"precip_pct_max"`
	Code         int     `json:"code"`
}

type weatherCurrent struct {
	Temp        float64 `json:"temp"`
	FeelsLike   float64 `json:"feels_like"`
	Description string  `json:"description"`
	WindKmh     float64 `json:"wind_kmh"`
	WindDir     string  `json:"wind_dir"`
	Code        int     `json:"code"`
}

type weatherHour struct {
	Hour      string  `json:"hour"`
	Temp      float64 `json:"temp"`
	PrecipPct int     `json:"precip_pct,omitempty"`
	Code      int     `json:"code"`
}

func fetchWeather(lat, lon float64, tzName string) (any, error) {
	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%v&longitude=%v"+
			"&current=temperature_2m,apparent_temperature,weather_code,wind_speed_10m,wind_direction_10m"+
			"&hourly=temperature_2m,precipitation_probability,weather_code"+
			"&daily=temperature_2m_max,temperature_2m_min,weather_code,precipitation_probability_max"+
			"&timezone=%s&forecast_days=3",
		lat, lon, urlEncode(tzName),
	)
	var r openMeteoResp
	if err := httpGetJSON(url, &r); err != nil {
		return nil, err
	}

	tz, err := time.LoadLocation(tzName)
	if err != nil {
		tz = time.UTC
	}
	nowTor := time.Now().In(tz)
	startTime := nowTor.Truncate(time.Hour).Add(time.Hour)

	out := weatherOut{}
	out.Current.Temp = r.Current.Temp
	out.Current.FeelsLike = r.Current.ApparentTemp
	out.Current.Description = wmoCodeName(r.Current.WeatherCode)
	out.Current.WindKmh = r.Current.WindSpeed
	out.Current.WindDir = compassDir(r.Current.WindDirection)
	out.Current.Code = r.Current.WeatherCode

	for i, ts := range r.Hourly.Time {
		if i >= len(r.Hourly.Temperature) {
			break
		}
		parsed, err := time.ParseInLocation("2006-01-02T15:04", ts, tz)
		if err != nil || parsed.Before(startTime) {
			continue
		}
		if len(out.Hourly) >= 24 {
			break
		}
		h := weatherHour{
			Hour: fmt.Sprintf("%02d", parsed.Hour()),
			Temp: r.Hourly.Temperature[i],
		}
		if i < len(r.Hourly.PrecipProb) {
			h.PrecipPct = r.Hourly.PrecipProb[i]
		}
		if i < len(r.Hourly.WeatherCode) {
			h.Code = r.Hourly.WeatherCode[i]
		}
		out.Hourly = append(out.Hourly, h)
	}

	out.Summary = summarizeWeather(out.Current.Description, out.Hourly)

	if len(r.Daily.Time) >= 2 {
		t := &weatherTomorrow{}
		if len(r.Daily.TempMax) >= 2 {
			t.HighC = r.Daily.TempMax[1]
		}
		if len(r.Daily.TempMin) >= 2 {
			t.LowC = r.Daily.TempMin[1]
		}
		if len(r.Daily.WeatherCode) >= 2 {
			t.Description = wmoCodeName(r.Daily.WeatherCode[1])
			t.Code = r.Daily.WeatherCode[1]
		}
		if len(r.Daily.PrecipProbMax) >= 2 {
			t.PrecipPctMax = r.Daily.PrecipProbMax[1]
		}
		out.Tomorrow = t
	}

	return out, nil
}

func summarizeWeather(desc string, hourly []weatherHour) string {
	for _, h := range hourly {
		if h.PrecipPct >= 50 {
			return fmt.Sprintf("%s. Rain likely from %s:00.", desc, h.Hour)
		}
	}
	return desc + "."
}

func compassDir(deg float64) string {
	dirs := []string{"N", "NNE", "NE", "ENE", "E", "ESE", "SE", "SSE",
		"S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW"}
	idx := int(math.Mod(math.Round(deg/22.5), 16))
	if idx < 0 {
		idx += 16
	}
	return dirs[idx]
}

func wmoCodeName(code int) string {
	m := map[int]string{
		0: "Clear sky", 1: "Mainly clear", 2: "Partly cloudy", 3: "Overcast",
		45: "Foggy", 48: "Foggy with rime",
		51: "Light drizzle", 53: "Drizzle", 55: "Heavy drizzle",
		56: "Light freezing drizzle", 57: "Freezing drizzle",
		61: "Light rain", 63: "Rain", 65: "Heavy rain",
		66: "Light freezing rain", 67: "Freezing rain",
		71: "Light snow", 73: "Snow", 75: "Heavy snow",
		77: "Snow grains",
		80: "Light showers", 81: "Showers", 82: "Violent showers",
		85: "Snow showers", 86: "Heavy snow showers",
		95: "Thunderstorm", 96: "Thunderstorm with hail", 99: "Severe thunderstorm",
	}
	if name, ok := m[code]; ok {
		return name
	}
	return "Unknown"
}

type aqhiAPIResp struct {
	Features []struct {
		Properties struct {
			AQHI                float64 `json:"aqhi"`
			LocationID          string  `json:"location_id"`
			LocationNameEn      string  `json:"location_name_en"`
			ObservationDatetime string  `json:"observation_datetime"`
		} `json:"properties"`
	} `json:"features"`
}

type aqhiOut struct {
	Value    int       `json:"value"`
	Raw      float64   `json:"raw"`
	Band     string    `json:"band"`
	Scale    string    `json:"scale"`
	Station  string    `json:"station"`
	Observed time.Time `json:"observed"`
}

type openMeteoAQResp struct {
	Current struct {
		Time         string  `json:"time"`
		EuropeanAQI  float64 `json:"european_aqi"`
	} `json:"current"`
}

// Canadian ECCC AQHI if the location is in Canada, otherwise European AQI from Open-Meteo (global).
func fetchAQHI(lat, lon float64) (any, error) {
	if out, err := fetchECCCAQHI(lat, lon); err == nil {
		return out, nil
	}
	return fetchOpenMeteoAQ(lat, lon)
}

func fetchECCCAQHI(lat, lon float64) (any, error) {
	url := fmt.Sprintf(
		"https://api.weather.gc.ca/collections/aqhi-observations-realtime/items"+
			"?bbox=%f,%f,%f,%f&limit=100&f=json",
		lon-0.4, lat-0.3, lon+0.4, lat+0.3,
	)
	var r aqhiAPIResp
	if err := httpGetJSON(url, &r); err != nil {
		return nil, err
	}
	if len(r.Features) == 0 {
		return nil, fmt.Errorf("no ECCC stations in bbox")
	}

	var newestIdx int
	var newestTime time.Time
	for i, f := range r.Features {
		t, err := time.Parse(time.RFC3339, f.Properties.ObservationDatetime)
		if err != nil {
			continue
		}
		if t.After(newestTime) {
			newestTime = t
			newestIdx = i
		}
	}

	p := r.Features[newestIdx].Properties
	rounded := max(int(math.Round(p.AQHI)), 1)
	band := "low"
	switch {
	case rounded <= 3:
		band = "low"
	case rounded <= 6:
		band = "moderate"
	case rounded <= 10:
		band = "high"
	default:
		band = "very high"
	}
	return aqhiOut{
		Value:    rounded,
		Raw:      p.AQHI,
		Band:     band,
		Scale:    "AQHI",
		Station:  p.LocationNameEn,
		Observed: newestTime,
	}, nil
}

func fetchOpenMeteoAQ(lat, lon float64) (any, error) {
	url := fmt.Sprintf(
		"https://air-quality-api.open-meteo.com/v1/air-quality?latitude=%v&longitude=%v&current=european_aqi&timezone=auto",
		lat, lon,
	)
	var r openMeteoAQResp
	if err := httpGetJSON(url, &r); err != nil {
		return nil, err
	}
	rounded := int(math.Round(r.Current.EuropeanAQI))
	band := "good"
	switch {
	case rounded <= 20:
		band = "good"
	case rounded <= 40:
		band = "fair"
	case rounded <= 60:
		band = "moderate"
	case rounded <= 80:
		band = "poor"
	case rounded <= 100:
		band = "very poor"
	default:
		band = "extreme"
	}
	t, _ := time.Parse("2006-01-02T15:04", r.Current.Time)
	return aqhiOut{
		Value:    rounded,
		Raw:      r.Current.EuropeanAQI,
		Band:     band,
		Scale:    "EAQI",
		Observed: t,
	}, nil
}

type rssEnvelope struct {
	Channel struct {
		Items []struct {
			Title       string   `xml:"title"`
			Link        string   `xml:"link"`
			PubDate     string   `xml:"pubDate"`
			Description string   `xml:"description"`
			Source      string   `xml:"source"`
			GUID        string   `xml:"guid"`
			Categories  []string `xml:"category"`
		} `xml:"item"`
	} `xml:"channel"`
}

type feedItem struct {
	GUID       string    `json:"guid"`
	Title      string    `json:"title"`
	Link       string    `json:"link"`
	PubDate    time.Time `json:"pub_date"`
	Source     string    `json:"source"`
	Categories []string  `json:"categories,omitempty"`
}

func fetchFeed() (any, error) {
	const url = "https://omnomfeeds.com/feed.xml"
	var env rssEnvelope
	if err := httpGetXML(url, &env); err != nil {
		return nil, err
	}
	items := make([]feedItem, 0, len(env.Channel.Items))
	seen := map[string]bool{}
	for _, it := range env.Channel.Items {
		if it.GUID != "" && seen[it.GUID] {
			continue
		}
		if it.GUID != "" {
			seen[it.GUID] = true
		}
		pub, _ := parseRSSDate(it.PubDate)
		items = append(items, feedItem{
			GUID:       it.GUID,
			Title:      strings.TrimSpace(it.Title),
			Link:       it.Link,
			PubDate:    pub,
			Source:     strings.TrimSpace(it.Source),
			Categories: it.Categories,
		})
	}
	sortByDateDesc(items)
	if len(items) > 50 {
		items = items[:50]
	}
	return items, nil
}

func parseRSSDate(s string) (time.Time, error) {
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 MST",
		"Mon, 2 Jan 2006 15:04:05 MST",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("bad date: %q", s)
}

func sortByDateDesc(items []feedItem) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].PubDate.After(items[j-1].PubDate); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

type astroOut struct {
	Sun  astroSun  `json:"sun"`
	Moon astroMoon `json:"moon"`
}

type astroSun struct {
	Rise     string  `json:"rise"`
	Set      string  `json:"set"`
	RiseUnix int64   `json:"rise_unix"`
	SetUnix  int64   `json:"set_unix"`
	NowPct   float64 `json:"now_pct"`
}

type astroMoon struct {
	Phase    string `json:"phase"`
	IllumPct int    `json:"illum_pct"`
	Rise     string `json:"rise"`
	Set      string `json:"set"`
	RiseUnix int64  `json:"rise_unix"`
	SetUnix  int64  `json:"set_unix"`
}

func fetchAstro(lat, lon float64, tzName string) (any, error) {
	now := time.Now()
	tz, err := time.LoadLocation(tzName)
	if err != nil {
		tz = time.UTC
	}

	rise, set := sunRiseSet(now, lat, lon)
	nowPct := 0.0
	if !rise.IsZero() && !set.IsZero() {
		if now.After(rise) && now.Before(set) {
			total := set.Sub(rise).Seconds()
			elapsed := now.Sub(rise).Seconds()
			if total > 0 {
				nowPct = elapsed / total
			}
		}
	}

	phase, illum := moonPhase(now)
	mRise, mSet := moonRiseSet(now, lat, lon)

	return astroOut{
		Sun: astroSun{
			Rise:     safeFormat(rise, tz),
			Set:      safeFormat(set, tz),
			RiseUnix: unixOrZero(rise),
			SetUnix:  unixOrZero(set),
			NowPct:   nowPct,
		},
		Moon: astroMoon{
			Phase:    phase,
			IllumPct: illum,
			Rise:     safeFormat(mRise, tz),
			Set:      safeFormat(mSet, tz),
			RiseUnix: unixOrZero(mRise),
			SetUnix:  unixOrZero(mSet),
		},
	}, nil
}

func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// The moon's visibility window often crosses midnight, so we pick the cycle that surrounds t.
func moonRiseSet(t time.Time, lat, lon float64) (rise, set time.Time) {
	type pair struct{ rise, set time.Time }
	var pairs []pair
	for _, d := range []time.Time{t.Add(-24 * time.Hour), t, t.Add(24 * time.Hour)} {
		r, s := computeMoonRiseSet(d, lat, lon)
		if !r.IsZero() && !s.IsZero() {
			pairs = append(pairs, pair{r, s})
		}
	}
	// Surrounds now?
	for _, p := range pairs {
		if (t.After(p.rise) || t.Equal(p.rise)) && t.Before(p.set) {
			return p.rise, p.set
		}
	}
	// Next upcoming?
	for _, p := range pairs {
		if p.rise.After(t) {
			return p.rise, p.set
		}
	}
	if len(pairs) > 0 {
		return pairs[len(pairs)-1].rise, pairs[len(pairs)-1].set
	}
	return time.Time{}, time.Time{}
}

// Approximation: moon lags sun by (age / synodic) * 24h. Accurate to ~30 min.
func computeMoonRiseSet(t time.Time, lat, lon float64) (rise, set time.Time) {
	sRise, sSet := sunRiseSet(t, lat, lon)
	if sRise.IsZero() || sSet.IsZero() {
		return time.Time{}, time.Time{}
	}
	age, synodic := moonAge(t)
	lag := time.Duration((age / synodic) * 24 * float64(time.Hour))
	return sRise.Add(lag), sSet.Add(lag)
}

func moonAge(t time.Time) (float64, float64) {
	const ref = 2451550.1
	const synodic = 29.530588853
	y, m, d := t.UTC().Date()
	jd := julianDay(y, int(m), d) + float64(t.UTC().Hour())/24.0 + float64(t.UTC().Minute())/1440.0
	age := math.Mod(jd-ref, synodic)
	if age < 0 {
		age += synodic
	}
	return age, synodic
}

func safeFormat(t time.Time, tz *time.Location) string {
	if t.IsZero() {
		return "--:--"
	}
	return t.In(tz).Format("15:04")
}

// NOAA solar position math; n must be the integer solar-noon-day count since J2000.
func sunRiseSet(t time.Time, lat, lon float64) (rise, set time.Time) {
	y, m, d := t.UTC().Date()
	jd := julianDay(y, int(m), d)

	n := math.Ceil(jd - 2451545.0 + 0.0008)
	jStar := n - lon/360.0
	M := math.Mod(357.5291+0.98560028*jStar, 360.0)
	mRad := deg2rad(M)
	C := 1.9148*math.Sin(mRad) + 0.0200*math.Sin(2*mRad) + 0.0003*math.Sin(3*mRad)
	lambda := math.Mod(M+C+180+102.9372, 360.0)
	lamRad := deg2rad(lambda)
	jTransit := 2451545.0 + jStar + 0.0053*math.Sin(mRad) - 0.0069*math.Sin(2*lamRad)
	delta := rad2deg(math.Asin(math.Sin(lamRad) * math.Sin(deg2rad(23.4397))))
	dRad := deg2rad(delta)
	latRad := deg2rad(lat)
	cosH := (math.Sin(deg2rad(-0.83)) - math.Sin(latRad)*math.Sin(dRad)) / (math.Cos(latRad) * math.Cos(dRad))
	if cosH < -1 || cosH > 1 {
		return time.Time{}, time.Time{}
	}
	H := rad2deg(math.Acos(cosH))
	jSet := jTransit + H/360.0
	jRise := jTransit - H/360.0
	return julianToTime(jRise), julianToTime(jSet)
}

func julianDay(y, m, d int) float64 {
	if m <= 2 {
		y--
		m += 12
	}
	A := y / 100
	B := 2 - A + A/4
	return math.Floor(365.25*float64(y+4716)) + math.Floor(30.6001*float64(m+1)) + float64(d) + float64(B) - 1524.5
}

func julianToTime(jd float64) time.Time {
	secs := (jd - 2440587.5) * 86400
	return time.Unix(int64(secs), 0).UTC()
}

func deg2rad(d float64) float64 { return d * math.Pi / 180 }
func rad2deg(r float64) float64 { return r * 180 / math.Pi }

// moonPhase: synodic period from known new moon J2000.
func moonPhase(t time.Time) (string, int) {
	age, synodic := moonAge(t)
	fraction := age / synodic
	illum := int(math.Round(50 * (1 - math.Cos(2*math.Pi*fraction))))
	switch {
	case fraction < 0.03 || fraction > 0.97:
		return "New", illum
	case fraction < 0.22:
		return "Waxing crescent", illum
	case fraction < 0.28:
		return "First quarter", illum
	case fraction < 0.47:
		return "Waxing gibbous", illum
	case fraction < 0.53:
		return "Full", illum
	case fraction < 0.72:
		return "Waning gibbous", illum
	case fraction < 0.78:
		return "Last quarter", illum
	default:
		return "Waning crescent", illum
	}
}

type biling struct {
	En string `json:"en"`
}

type cityPageResp struct {
	Features []struct {
		Properties struct {
			Name     biling `json:"name"`
			Warnings []struct {
				Type        biling `json:"type"`
				EventIssue  biling `json:"eventIssue"`
				Description biling `json:"description"`
				ExpiryTime  biling `json:"expiryTime"`
				URL         biling `json:"url"`
			} `json:"warnings"`
		} `json:"properties"`
	} `json:"features"`
}

type alertOut struct {
	Type        string    `json:"type"`
	Event       string    `json:"event"`
	Description string    `json:"description"`
	Body        string    `json:"body"`
	Expires     time.Time `json:"expires"`
	URL         string    `json:"url"`
	Locations   []string  `json:"locations"`
}

func fetchAlerts(lat, lon float64) (any, error) {
	url := fmt.Sprintf(
		"https://api.weather.gc.ca/collections/citypageweather-realtime/items"+
			"?bbox=%f,%f,%f,%f&limit=20&f=json",
		lon-0.4, lat-0.3, lon+0.4, lat+0.3,
	)
	var r cityPageResp
	if err := httpGetJSON(url, &r); err != nil {
		return nil, err
	}
	byKey := map[string]*alertOut{}
	for _, f := range r.Features {
		loc := f.Properties.Name.En
		for _, w := range f.Properties.Warnings {
			key := w.Description.En
			if key == "" {
				continue
			}
			exp, _ := time.Parse(time.RFC3339, w.ExpiryTime.En)
			if a, ok := byKey[key]; ok {
				a.Locations = append(a.Locations, loc)
				continue
			}
			byKey[key] = &alertOut{
				Type:        w.Type.En,
				Event:       w.EventIssue.En,
				Description: w.Description.En,
				Expires:     exp,
				URL:         w.URL.En,
				Locations:   []string{loc},
			}
		}
	}
	out := make([]*alertOut, 0, len(byKey))
	for _, a := range byKey {
		out = append(out, a)
	}
	var wg sync.WaitGroup
	for _, a := range out {
		wg.Add(1)
		go func(a *alertOut) {
			defer wg.Done()
			a.Body = fetchAlertBody(a.URL, a.Description)
		}(a)
	}
	wg.Wait()
	return out, nil
}

var (
	htmlTagRE   = regexp.MustCompile(`<[^>]*>`)
	timestampRE = regexp.MustCompile(`\d{1,2}:\d{2}\s*(?:AM|PM)?\s+[A-Z]{2,4}\s+\w+\s+\d+\s+\w+\s+\d{4}\.?`)
	whitespaceRE = regexp.MustCompile(`\s+`)
)

func fetchAlertBody(reportURL, headline string) string {
	if reportURL == "" || headline == "" {
		return ""
	}
	req, err := http.NewRequest("GET", reportURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return ""
	}
	return extractAlertBody(string(b), headline)
}

func extractAlertBody(htmlText, headline string) string {
	lower := strings.ToLower(htmlText)
	needle := strings.ToLower(headline)
	idx := strings.Index(lower, needle)
	if idx == -1 {
		return ""
	}
	chunk := htmlText[idx+len(headline):]
	if len(chunk) > 6000 {
		chunk = chunk[:6000]
	}
	text := htmlTagRE.ReplaceAllString(chunk, " ")
	text = html.UnescapeString(text)
	if m := timestampRE.FindStringIndex(text); m != nil {
		text = text[m[1]:]
	}
	text = whitespaceRE.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	for _, cut := range []string{"Additional information:", "Please continue to monitor", "To report severe weather"} {
		if i := strings.Index(text, cut); i > 40 {
			text = text[:i]
		}
	}
	text = strings.TrimSpace(text)
	if len(text) > 800 {
		text = text[:800] + " ..."
	}
	return text
}

var (
	cWeather *sourceState
	cAQHI    *sourceState
	cAstro   *sourceState
	cFeed    *sourceState
	cAlerts  *sourceState
)

func main() {
	configPath := flag.String("config", "config.json", "path to config.json")
	portOverride := flag.Int("port", 0, "override listen port from config")
	flag.Parse()

	cfg := loadConfig(*configPath)
	if *portOverride != 0 {
		cfg.Port = *portOverride
	}
	port := &cfg.Port

	cWeather = newCache("weather", 10*time.Minute, func() (any, error) {
		return fetchWeather(cfg.PrimaryLat, cfg.PrimaryLon, cfg.PrimaryTZ)
	})
	cAQHI = newCache("aqhi", 60*time.Minute, func() (any, error) {
		return fetchAQHI(cfg.PrimaryLat, cfg.PrimaryLon)
	})
	cAstro = newCache("astro", 6*time.Hour, func() (any, error) {
		return fetchAstro(cfg.PrimaryLat, cfg.PrimaryLon, cfg.PrimaryTZ)
	})
	cFeed = newCache("feed", 15*time.Minute, fetchFeed)
	cAlerts = newCache("alerts", 10*time.Minute, func() (any, error) {
		return fetchAlerts(cfg.PrimaryLat, cfg.PrimaryLon)
	})

	mux := http.NewServeMux()

	web, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(web)))

	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/weather", apiHandler(cWeather))
	mux.HandleFunc("/api/aqhi", apiHandler(cAQHI))
	mux.HandleFunc("/api/astro", apiHandler(cAstro))
	mux.HandleFunc("/api/feed", apiHandler(cFeed))
	mux.HandleFunc("/api/alerts", apiHandler(cAlerts))
	mux.HandleFunc("/api/config", handleConfigGet(&cfg))
	mux.HandleFunc("/config", handleConfigPage(*configPath, &cfg))

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	log.Printf("xeneon-dash listening on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func apiHandler(c *sourceState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reply := c.get()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Source-Status", reply.Status)
		if reply.AgeSec > 0 {
			w.Header().Set("X-Cache-Age", fmt.Sprintf("%d", reply.AgeSec))
		}
		if reply.Payload == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			io.WriteString(w, fmt.Sprintf(`{"error":%q}`, reply.Error))
			return
		}
		json.NewEncoder(w).Encode(reply.Payload)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	sources := map[string]any{}
	for name, s := range map[string]*sourceState{
		"weather": cWeather, "aqhi": cAQHI, "astro": cAstro, "feed": cFeed,
	} {
		s.mu.Lock()
		ss := map[string]any{
			"has_payload": s.payload != nil,
			"failures":    s.failures,
			"last_error":  s.lastError,
		}
		if !s.lastSuccess.IsZero() {
			ss["last_success"] = s.lastSuccess.UTC()
			ss["age_sec"] = int(time.Since(s.lastSuccess).Seconds())
		}
		s.mu.Unlock()
		sources[name] = ss
	}
	writeJSON(w, map[string]any{"status": "ok", "sources": sources, "time": time.Now().UTC()})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func urlEncode(s string) string {
	return strings.ReplaceAll(s, "/", "%2F")
}

func handleConfigGet(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"primary": map[string]any{
				"label": cfg.PrimaryLabel,
				"lat":   cfg.PrimaryLat,
				"lon":   cfg.PrimaryLon,
				"tz":    cfg.PrimaryTZ,
			},
			"secondary": map[string]any{
				"label": cfg.SecondaryLabel,
				"tz":    cfg.SecondaryTZ,
			},
		})
	}
}

func handleConfigPage(path string, cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			newCfg := *cfg
			if v := r.FormValue("primary_label"); v != "" {
				newCfg.PrimaryLabel = v
			}
			if v := r.FormValue("primary_lat"); v != "" {
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					newCfg.PrimaryLat = f
				}
			}
			if v := r.FormValue("primary_lon"); v != "" {
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					newCfg.PrimaryLon = f
				}
			}
			if v := r.FormValue("primary_tz"); v != "" {
				if _, err := time.LoadLocation(v); err == nil {
					newCfg.PrimaryTZ = v
				}
			}
			if v := r.FormValue("secondary_label"); v != "" {
				newCfg.SecondaryLabel = v
			}
			if v := r.FormValue("secondary_tz"); v != "" {
				if _, err := time.LoadLocation(v); err == nil {
					newCfg.SecondaryTZ = v
				}
			}
			if err := saveConfig(path, newCfg); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			*cfg = newCfg
			log.Printf("[config] saved to %s, restart server to apply", path)
			http.Redirect(w, r, "/config?saved=1", http.StatusSeeOther)
			return
		}
		renderConfigForm(w, *cfg, r.URL.Query().Get("saved") == "1")
	}
}

func renderConfigForm(w http.ResponseWriter, c Config, saved bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl := `<!DOCTYPE html><html><head><meta charset="utf-8">
<title>xeneon-dash . config</title>
<link rel="stylesheet" href="/dash.css">
<style>
  body { width: auto; height: auto; overflow: auto; padding: 40px; }
  .config-card {
    max-width: 720px;
    margin: 0 auto;
    background: rgba(0, 30, 50, 0.4);
    border: 1px solid rgba(0, 180, 255, 0.3);
    padding: 28px 36px;
    box-shadow: inset 0 0 24px rgba(0, 180, 255, 0.06);
  }
  h1 { font-size: 28px; letter-spacing: 4px; color: #d68a3a; margin-bottom: 6px; text-transform: uppercase; }
  h2 { font-size: 14px; letter-spacing: 3px; color: #d68a3a; text-transform: uppercase; margin: 28px 0 10px; opacity: 0.85; }
  label { display: block; font-size: 13px; letter-spacing: 1px; color: #8aa9c4; text-transform: uppercase; margin: 14px 0 4px; }
  input {
    width: 100%; padding: 10px 12px; font-size: 16px;
    background: #050810; color: #cfe7ff;
    border: 1px solid rgba(0, 180, 255, 0.3);
    font-family: 'JetBrains Mono', 'Consolas', monospace;
  }
  input:focus { outline: none; border-color: rgba(255, 122, 0, 0.6); }
  .row { display: flex; gap: 14px; }
  .row > div { flex: 1; }
  button {
    margin-top: 24px; padding: 12px 28px; font-size: 15px;
    background: rgba(255, 122, 0, 0.18);
    border: 1px solid rgba(255, 122, 0, 0.7);
    color: #ffb74d;
    letter-spacing: 2px;
    text-transform: uppercase;
    cursor: pointer;
    font-family: 'JetBrains Mono', 'Consolas', monospace;
  }
  button:hover { background: rgba(255, 122, 0, 0.28); }
  .hint { font-size: 12px; color: #7aa9c9; margin-top: 4px; }
  .saved {
    background: rgba(108, 209, 108, 0.15);
    border: 1px solid rgba(108, 209, 108, 0.6);
    color: #6cd16c; padding: 10px 14px; margin-bottom: 24px;
  }
  a { color: #ff7a00; }
</style>
</head><body>
<div class="config-card">
<h1>xeneon-dash . config</h1>
{{if .Saved}}<div class="saved">Saved to config.json. Restart xeneon-dash for changes to take effect.</div>{{end}}
<form method="post">
<h2>Primary location . drives weather, astro, aqhi, alerts</h2>
<label>City label (display name)</label>
<input name="primary_label" value="{{.C.PrimaryLabel}}">
<div class="row">
<div>
<label>Latitude</label>
<input name="primary_lat" value="{{.C.PrimaryLat}}" type="number" step="any">
</div><div>
<label>Longitude</label>
<input name="primary_lon" value="{{.C.PrimaryLon}}" type="number" step="any">
</div></div>
<p class="hint">Look up at <a href="https://www.latlong.net/" target="_blank">latlong.net</a>. AQHI and ECCC alerts only work for Canadian locations.</p>
<label>IANA timezone</label>
<input name="primary_tz" value="{{.C.PrimaryTZ}}">
<p class="hint">e.g. America/Toronto, Europe/London, Australia/Adelaide. Full list: <a href="https://en.wikipedia.org/wiki/List_of_tz_database_time_zones" target="_blank">tz database</a></p>
<h2>Secondary clock</h2>
<label>City label</label>
<input name="secondary_label" value="{{.C.SecondaryLabel}}">
<label>IANA timezone</label>
<input name="secondary_tz" value="{{.C.SecondaryTZ}}">
<button type="submit">Save</button>
</form>
<p class="hint" style="margin-top:30px"><a href="/">. back to dashboard</a></p>
</div></body></html>`
	t, err := template.New("config").Parse(tmpl)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	_ = t.Execute(w, map[string]any{"C": c, "Saved": saved})
}

