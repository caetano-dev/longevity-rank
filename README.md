# Longevity Ranker

## TODO:
- Solve inconsistencies in the data (e.g. Do Not Age / Pure NMN (366 Capsules)/ Capsules / $440.00 / 183.0g / $2.40) - The calculated gram is wrong. It is calculating for 30 capsules only.
- Add more vendors
- Add more products (creatine, TMG, etc.)


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