package service

import (
	"math"
	"testing"
)

func TestValidateCustomerLocation(t *testing.T) {
	valid := CustomerLocationInput{
		Latitude: 31.230416, Longitude: 121.473701, AccuracyM: 18.5,
	}
	if err := validateCustomerLocation(valid); err != nil {
		t.Fatalf("valid location rejected: %v", err)
	}

	invalid := []CustomerLocationInput{
		{Latitude: 91, Longitude: 0, AccuracyM: 1},
		{Latitude: 0, Longitude: -181, AccuracyM: 1},
		{Latitude: 0, Longitude: 0, AccuracyM: -1},
		{Latitude: math.NaN(), Longitude: 0, AccuracyM: 1},
	}
	for _, input := range invalid {
		if err := validateCustomerLocation(input); err == nil {
			t.Fatalf("invalid location accepted: %+v", input)
		}
	}
}
