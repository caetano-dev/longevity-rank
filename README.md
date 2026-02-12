# Longevity Ranker

## TODO:
- Add more vendors
- Solve inconsistencies in the data (e.g. Do Not Age / Pure NMN (366 Capsules)/ Capsules / $440.00 / 183.0g / $2.40) - The calculated gram is wrong. It is calculating for 30 capsules only.
- Add more products (creatine, TMG, etc.)
- Fix NMN bio currency. Adding ?currency=USD at the end make the code fetch a bunch of pages non-stop.
- Fix cloudflare websites

* **Feature: Affiliate Link Generation**
* **Requirement:** "GeniusLink or simple JS redirection logic."
* **Current Status:** You have the product `Handle` (URL slug) and the vendor's base URL.
* **Implementation:** You can add an `AffiliateID` field to your `VendorConfig` in `rules.go`. In `main.go`, you can concatenate `Vendor URL` + `Handle` + `?ref=AffiliateID` to generate the final "Buy Now" link for the frontend.

## Setup

```go
go mod tidy
```

## Usage

Scrape data and rank products
```go
go run cmd/main.go -refresh
```

Rank products with local data
```go
go run cmd/main.go
```