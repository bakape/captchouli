package db

import "testing"

func TestUpkeep(t *testing.T) {
	err := deleteStaleCaptchas()
	if err != nil {
		t.Fatal(err)
	}
	err = vacuum()
	if err != nil {
		t.Fatal(err)
	}
}
