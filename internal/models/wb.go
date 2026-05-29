package models

type WBProductResult struct {
	ID            int
	ReviewRating  float64
	TotalQuantity int
	Price         int
}

type WBResultData struct {
	Products []WBProductResult
}
