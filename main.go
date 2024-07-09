package main

import (
	"fmt"
	"training/internal/workerpool" // путь к вашему пакету examples
)

func main() {
	// Вызов примера MainWorkerpool
	fmt.Println("=== MainWorkerpool() ===")
	workerpool.MainWorkerpool()
}
