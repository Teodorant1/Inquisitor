package main

import (
	"fmt"
	"math/rand"
	"time"

	// Using the exact same package/library as your code
	"github.com/jung-kurt/gofpdf"
)

// GenerateMultiPageTestPDF creates a sample PDF with N random math questions
func GenerateMultiPageTestPDF(filename string, questionCount int) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(10, 10, 10)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	// Set standard font matching your primary file fallback
	pdf.SetFont("Arial", "", 12)

	// Seed random number generator
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	operators := []string{"+", "-", "*", "/"}

	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(0, 10, "Generated Multi-Page Test Document", "", 1, "C", false, 0, "")
	pdf.Ln(5)

	pdf.SetFont("Arial", "", 11)

	for i := 1; i <= questionCount; i++ {
		num1 := r.Intn(100) + 1
		num2 := r.Intn(50) + 1
		op := operators[r.Intn(len(operators))]

		// Create a mock question entry
		questionText := fmt.Sprintf("Question %d: What is the result of %d %s %d? Explain your reasoning in detail.", i, num1, op, num2)

		// Print question text
		pdf.MultiCell(0, 6, questionText, "", "L", false)
		pdf.Ln(2)

		// Draw separation line
		currentY := pdf.GetY()
		pdf.SetDrawColor(200, 200, 200)
		pdf.Line(10, currentY, 200, currentY)

		// Add vertical spacing simulating answer area
		pdf.Ln(12)
	}

	if pdf.Err() {
		return fmt.Errorf("pdf generation error: %v", pdf.Error())
	}

	return pdf.OutputFileAndClose(filename)
}

func main() {
	outputFile := "multipage_sample.pdf"
	// 200 questions should create a long multi-page document
	err := GenerateMultiPageTestPDF(outputFile, 200) 
	if err != nil {
		fmt.Printf("Error creating test file: %v\n", err)
		return
	}

	fmt.Printf("Successfully created multi-page test file: %s\n", outputFile)
}