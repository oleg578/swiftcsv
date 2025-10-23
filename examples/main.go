package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/oleg578/swiftcsv"
)

func newCSVReader(src io.Reader) *swiftcsv.Reader {
	return swiftcsv.NewReader(src)
}

func main() {
	// Open the CSV file
	file, err := os.Open("dummy.csv")
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer file.Close()

	// Create a buffered reader
	bufReader := bufio.NewReader(file)

	// Create a CSV reader
	csvReader := newCSVReader(bufReader)

	// Read the header row
	fmt.Println("=== CSV Reader Example ===")
	fmt.Println("Reading header row:")
	header, err := csvReader.Read()
	if err != nil {
		fmt.Printf("Error reading header: %v\n", err)
		return
	}

	// Print header with column numbers
	for i, field := range header {
		fmt.Printf("Column %d: %s\n", i+1, field)
	}

	fmt.Println("\n=== First 10 Data Rows ===")

	// Read and display the first 10 data rows
	rowCount := 0
	for rowCount < 10 {
		record, err := csvReader.Read()
		if err == io.EOF {
			fmt.Println("Reached end of file")
			break
		}
		if err != nil {
			fmt.Printf("Error reading record: %v\n", err)
			break
		}

		rowCount++
		fmt.Printf("Row %d:\n", rowCount)
		for i, field := range record {
			if i < len(header) {
				fmt.Printf("  %s: %s\n", header[i], field)
			} else {
				fmt.Printf("  Column %d: %s\n", i+1, field)
			}
		}
		fmt.Println()
	}

	// Example of processing specific fields
	fmt.Println("=== Processing Specific Fields ===")
	fmt.Println("Products with price > 50:")

	// Reset file pointer to read from beginning again
	file.Seek(0, 0)
	bufReader = bufio.NewReader(file)
	csvReader = newCSVReader(bufReader)

	// Skip header
	csvReader.Read()

	// Process all records to find expensive products
	expensiveCount := 0
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("Error reading record: %v\n", err)
			break
		}

		// Assuming price is in column 4 (index 3)
		if len(record) >= 4 {
			price := record[3]
			product := record[1]

			// Simple string comparison (in real code, you'd parse the float)
			if len(price) > 0 && price[0] >= '5' {
				expensiveCount++
				if expensiveCount <= 5 { // Show only first 5
					fmt.Printf("  %s - $%s\n", product, price)
				}
			}
		}
	}

	fmt.Printf("Found %d products with price > $50 (showing first 5)\n", expensiveCount)

	// Example with custom delimiter (semicolon)
	fmt.Println("\n=== Custom Delimiter Example ===")
	// Create some test data with semicolon delimiter
	testData := "name;age;city\nJohn;30;New York\nJane;25;Los Angeles\n"

	// Create reader from string
	stringReader := bufio.NewReader(strings.NewReader(testData))
	semicolonReader := newCSVReader(stringReader)
	semicolonReader.Comma = ';' // Set custom delimiter

	fmt.Println("Reading semicolon-delimited data:")
	for {
		record, err := semicolonReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			break
		}
		fmt.Printf("Record: %v\n", record)
	}
}
