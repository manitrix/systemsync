package process

import "testing"

func TestStatWithParentheses(t *testing.T) {
	ticks, start, e := ParseStat("42 (worker (pool)) S 1 2 3 4 5 6 7 8 9 10 100 50 0 0 0 0 2 0 12345 0")
	if e != nil || ticks != 150 || start != 12345 {
		t.Fatalf("ticks=%d start=%d err=%v", ticks, start, e)
	}
	if _, _, e = ParseStat("bad"); e == nil {
		t.Fatal("accepted malformed stat")
	}
}
