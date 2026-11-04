package main

import "testing"

func TestBookRowHitAndHighlightShareRuleBoundaries(t *testing.T) {
	for row := 0; row < booksPerPage(); row++ {
		r := bookRowRect(row)
		if row == 0 && r.Min.Y != headerH {
			t.Fatal("first row must meet header")
		}
		if row > 0 && r.Min.Y != bookRowRect(row-1).Max.Y+1 {
			t.Fatal("row must begin immediately below preceding separator")
		}
		for _, y := range []int{r.Min.Y, r.Max.Y - 1} {
			if bookRowAt(y) != row {
				t.Fatalf("highlight/hit mismatch row=%d y=%d", row, y)
			}
		}
		if r.Max.Y != listTop+row*rowH+separatorOffset {
			t.Fatal("highlight crosses separator")
		}
	}
}
