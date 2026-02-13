package parser

import "fmt"

// Static exchange rates to USD. Updated periodically via manual commit.
// This avoids runtime API calls and keeps the $0/month infrastructure promise.
var exchangeRates = map[string]float64{
	"USD": 1.0,
	"GBP": 1.27,
	"EUR": 1.09,
	"CAD": 0.74,
	"AUD": 0.66,
}

// ConvertToUSD converts a price from the given currency to USD.
// Returns the original price unchanged if currency is empty or "USD".
// Returns an error if the currency code is not in the static rate table.
func ConvertToUSD(price float64, currency string) (float64, error) {
	if currency == "" || currency == "USD" {
		return price, nil
	}

	rate, ok := exchangeRates[currency]
	if !ok {
		return 0, fmt.Errorf("unsupported currency %q — add it to exchangeRates in currency.go", currency)
	}

	return price * rate, nil
}