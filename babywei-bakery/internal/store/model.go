package store

// Purchase 是一筆進貨紀錄。
type Purchase struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Brand        string  `json:"brand"`
	PurchaseDate string  `json:"purchaseDate"`
	Channel      string  `json:"channel"`
	Price        float64 `json:"price"`
	WeightG      float64 `json:"weightG"`
}

// DoughItem 是產品配方中的一項材料，以 Baker's % 表示。
type DoughItem struct {
	Name string  `json:"name"`
	Pct  float64 `json:"pct"`
}

// Dough 是一份產品配方。
type Dough struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Ingredients []DoughItem `json:"ingredients"`
}

// FillingItem 是配料中的一項材料，以絕對克數表示。
type FillingItem struct {
	Name    string  `json:"name"`
	WeightG float64 `json:"weightG"`
}

// Filling 是一份配料配方。
type Filling struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Ingredients []FillingItem `json:"ingredients"`
}

// Product 是一項可販售商品。Fill1ID / Fill2ID 為空字串表示未使用該配料。
type Product struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Price        float64 `json:"price"`
	DoughID      string  `json:"doughId"`
	DoughWeightG float64 `json:"doughWeightG"`
	Fill1ID      string  `json:"fill1Id"`
	Fill1WeightG float64 `json:"fill1WeightG"`
	Fill2ID      string  `json:"fill2Id"`
	Fill2WeightG float64 `json:"fill2WeightG"`
}

// Sale 是一筆出貨紀錄。UnitCost 與 UnitPrice 是寫入當下的快照，
// 之後改配方或改售價都不影響已成立的紀錄。
type Sale struct {
	ID          string  `json:"id"`
	SaleDate    string  `json:"saleDate"`
	ProductID   string  `json:"productId"`
	ProductName string  `json:"productName"`
	Qty         int     `json:"qty"`
	UnitCost    float64 `json:"unitCost"`
	UnitPrice   float64 `json:"unitPrice"`
}

// Consumption 是一批生產對單一材料的消耗量，於確認生產時寫死。
type Consumption struct {
	IngredientName string  `json:"ingredientName"`
	ConsumedG      float64 `json:"consumedG"`
}

// ProductionLog 是一批生產紀錄。Consumption 是當下算出的原料消耗快照，
// 庫存一律以它為依據，不從當前配方回推。
type ProductionLog struct {
	ID          string        `json:"id"`
	LoggedDate  string        `json:"loggedDate"`
	ProductID   string        `json:"productId"`
	ProductName string        `json:"productName"`
	Qty         int           `json:"qty"`
	Consumption []Consumption `json:"consumption"`
}
