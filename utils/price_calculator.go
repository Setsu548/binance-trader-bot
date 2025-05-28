package utils

import (
	"math"
)

// CalculateBuyPrice calculates the limit price for a buy order.
// It returns the currentPrice reduced by the given percentage.
// Example: currentPrice = 100, percentage = 1.0 (1%) -> buyPrice = 99.0
func CalculateBuyPrice(currentPrice float64, percentage float64) float64 {
	// Ensure percentage is positive for reduction
	if percentage < 0 {
		percentage = -percentage
	}
	reductionFactor := 1.0 - (percentage / 100.0)
	return currentPrice * reductionFactor
}

// CalculateSellPrice calculates the limit price for a sell order.
// It returns the basePrice (e.g., actual buy price) increased by the given profit percentage.
// Example: buyPrice = 100, profitPercentage = 2.0 (2%) -> sellPrice = 102.0
func CalculateSellPrice(basePrice float64, profitPercentage float64) float64 {
	// Ensure profitPercentage is positive for increase
	if profitPercentage < 0 {
		profitPercentage = -profitPercentage
	}
	increaseFactor := 1.0 + (profitPercentage / 100.0)
	return basePrice * increaseFactor
}

// RoundToDecimalPlaces rounds a float64 to a specified number of decimal places.
// This is a basic rounding. For financial calculations, consider using decimal library
// as seen in binance_service.go, but this is fine for display or simpler internal calculations.
func RoundToDecimalPlaces(value float64, places int) float64 {
	shift := math.Pow(10, float64(places))
	return math.Round(value*shift) / shift
}
