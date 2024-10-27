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
	"fmt"
	"log"

	"github.com/joeychilson/uvgo"
)

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

func main() {
	ctx := context.Background()

	uv, err := uvgo.New(uvgo.WithDependencies("pandas"))
	if err != nil {
		log.Fatal("Failed to create runner:", err)
	}

	script := `
import pandas as pd
import json
from datetime import datetime

data = {
    'date': ['2024-01-01', '2024-01-15', '2024-02-01', '2024-02-15', '2024-03-01'],
    'product': ['Widget A', 'Widget B', 'Widget A', 'Widget C', 'Widget B'],
    'quantity': [10, 5, 8, 12, 6],
    'price': [100, 150, 100, 75, 150]
}

df = pd.DataFrame(data)
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

monthly_sales = df.groupby(df['date'].dt.strftime('%Y-%m'))['revenue'].sum().to_dict()

first_month = df[df['date'].dt.strftime('%Y-%m') == '2024-01']['revenue'].sum()
last_month = df[df['date'].dt.strftime('%Y-%m') == '2024-03']['revenue'].sum()
growth_rate = ((last_month - first_month) / first_month) * 100 if first_month > 0 else 0

analysis = {
    'total_revenue': float(total_revenue),
    'average_order_size': float(avg_order),
    'top_products': top_products,
    'monthly_sales': {k: float(v) for k, v in monthly_sales.items()},
    'growth_rate': float(growth_rate)
}

print(json.dumps(analysis))
`

	result, err := uvgo.StructuredOutputFromString[SalesAnalysis](ctx, uv, script)
	if err != nil {
		log.Fatalf("failed to run script: %v", err)
	}

	fmt.Printf("Duration: %s\n", result.Duration)

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

	if result.ExitCode != 0 {
		fmt.Printf("\nWarnings/Errors:\n%s\n", result.Stderr)
	}
}
```
