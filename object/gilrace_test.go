package object

// raceEnabled is set by the race build so latency-sensitive probes can skip
// their tail assertions where the detector's overhead would flake them.
var raceEnabled bool
