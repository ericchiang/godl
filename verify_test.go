package main

import "testing"

func TestDirSum(t *testing.T) {
	const expSum = "67bb1e266b75d4879fbc559c445b0b594d41ba28877b7cefebbbbb5be682adc0"

	sum, err := dirSum("testdata/verify-gold-standard")
	if err != nil {
		t.Fatal(err)
	}
	if sum != expSum {
		t.Fatalf("testdata sum did not match expected value")
	}
}
