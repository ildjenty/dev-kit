package report

import "testing"

func TestSum(t *testing.T) {
    if Sum([]Row{{Amount: 4}, {Amount: 6}}) != 10 { t.Fatal("sum changed") }
    if Sum(nil) != 0 { t.Fatal("empty sum changed") }
}
