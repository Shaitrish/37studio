package main

import (
	"testing"
	"time"
)

// Slots land on the chosen weekdays at local time, strictly after "now".
func TestNextSlot(t *testing.T) {
	dataDir = t.TempDir()
	openDB()
	dhaka, _ := time.LoadLocation("Asia/Dhaka")
	now := time.Date(2026, 10, 23, 21, 0, 0, 0, dhaka) // Friday 21:00
	cases := []struct {
		days, at string
		want     time.Time
	}{
		{"5", "20:00", time.Date(2026, 10, 30, 20, 0, 0, 0, dhaka)},  // today's 20:00 has passed: next Friday
		{"5", "22:30", time.Date(2026, 10, 23, 22, 30, 0, 0, dhaka)}, // later today
		{"0,2", "09:00", time.Date(2026, 10, 25, 9, 0, 0, 0, dhaka)}, // Sunday
	}
	for _, c := range cases {
		got, ok := nextSlot(c.days, c.at, now)
		if !ok || !got.Equal(c.want) {
			t.Errorf("nextSlot(%s, %s) = %v %v, want %v", c.days, c.at, got, ok, c.want)
		}
	}
	if _, ok := nextSlot("", "20:00", now); ok {
		t.Error("no days should mean no slot")
	}
}
