# uvgo

A library for running python scripts in Go using the [uv](https://docs.astral.sh/uv/) python package manager.


## Installation

```bash
go get -u github.com/joeychilson/uvgo
```

## Usage

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/joeychilson/uvgo"
)

type SalesData struct {
	Date     string  `json:"date"`
	Product  string  `json:"product"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

type SalesAnalysis struct {
	TotalRevenue     float64            `json:"total_revenue"`
	AverageOrderSize float64            `json:"average_order_size"`
	TopProducts      []ProductAnalysis  `json:"top_products"`
	MonthlySales     map[string]float64 `json:"monthly_sales"`
	Growth           float64            `json:"growth_rate"`
}

type ProductAnalysis struct {
	Name     string  `json:"name"`
	Revenue  float64 `json:"revenue"`
	Quantity int     `json:"quantity"`
}

func AnalyzeSalesScript(ctx context.Context, sales []SalesData) (string, error) {
	salesJSON, err := json.Marshal(sales)
	if err != nil {
		return "", fmt.Errorf("failed to marshal sales data: %w", err)
	}

	script := fmt.Sprintf(`
import pandas as pd
import json
from datetime import datetime


input_data = json.loads('''%s''')

df = pd.DataFrame(input_data)

df['date'] = pd.to_datetime(df['date'])
df['revenue'] = df['quantity'] * df['price']

total_revenue = df['revenue'].sum()
avg_order = df['revenue'].mean()

product_analysis = df.groupby('product').agg({
    'revenue': 'sum',
    'quantity': 'sum'
}).reset_index()

top_products = []
for _, row in product_analysis.iterrows():
    top_products.append({
        'name': row['product'],
        'revenue': float(row['revenue']),
        'quantity': int(row['quantity'])
    })

monthly_sales = df.groupby(df['date'].dt.strftime('%%Y-%%m'))['revenue'].sum().to_dict()

first_month = df[df['date'].dt.strftime('%%Y-%%m') == df['date'].dt.strftime('%%Y-%%m').min()]['revenue'].sum()
last_month = df[df['date'].dt.strftime('%%Y-%%m') == df['date'].dt.strftime('%%Y-%%m').max()]['revenue'].sum()
growth_rate = ((last_month - first_month) / first_month) * 100 if first_month > 0 else 0

analysis = {
    'total_revenue': float(total_revenue),
    'average_order_size': float(avg_order),
    'top_products': top_products,
    'monthly_sales': {k: float(v) for k, v in monthly_sales.items()},
    'growth_rate': float(growth_rate)
}

print(json.dumps(analysis))
`, strings.ReplaceAll(string(salesJSON), "'", "\\'"))

	return script, nil
}

func main() {
	salesData := []SalesData{
		{Date: "2024-01-01", Product: "Product A", Quantity: 10, Price: 100},
		{Date: "2024-01-15", Product: "Product B", Quantity: 5, Price: 150},
		{Date: "2024-02-01", Product: "Product A", Quantity: 8, Price: 100},
		{Date: "2024-02-15", Product: "Product C", Quantity: 12, Price: 75},
		{Date: "2024-03-01", Product: "Product B", Quantity: 6, Price: 150},
	}

	ctx := context.Background()
	script, err := AnalyzeSalesScript(ctx, salesData)
	if err != nil {
		log.Fatal(err)
	}

	uv, err := uvgo.New(uvgo.WithDependencies("pandas"))
	if err != nil {
		log.Fatal(err)
	}

	result, err := uvgo.StructuredOutputFromString[SalesAnalysis](ctx, uv, script)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Sales Analysis Results:\n")
	fmt.Printf("Total Revenue: $%.2f\n", result.Data.TotalRevenue)
	fmt.Printf("Average Order Size: $%.2f\n", result.Data.AverageOrderSize)
	fmt.Printf("Growth Rate: %.1f%%\n", result.Data.Growth)

	fmt.Printf("\nTop Products:\n")
	for _, product := range result.Data.TopProducts {
		fmt.Printf("- %s: $%.2f (Qty: %d)\n",
			product.Name,
			product.Revenue,
			product.Quantity,
		)
	}

	fmt.Printf("\nMonthly Sales:\n")
	for month, revenue := range result.Data.MonthlySales {
		fmt.Printf("- %s: $%.2f\n", month, revenue)
	}

	fmt.Printf("System Time: %s\n", result.SystemTime)
	fmt.Printf("User Time: %s\n", result.UserTime)
}
```
