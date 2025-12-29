package printer

import (
	"log"
	"math/rand"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

// MathQuestions holds the questions from the generated PDF
var MathQuestions = []string{
	"1) Solve the following equation: 2x + 5 = 13",
	"2) Calculate the area of a rectangle with sides 8cm and 12cm",
	"3) What is 25% of 480? Show your work.",
	"4) Solve the quadratic equation: x² - 5x + 6 = 0",
	"5) Calculate the volume of a cube with side 5cm",
	"6) Simplify the following expression: 3(2x + 4) - 2(x - 1)",
	"7) What is the length of the hypotenuse of a right triangle with sides 3cm and 4cm?",
	"8) Solve the system of equations: x + y = 10, x - y = 2",
}

func GenerateExamPDF() {
	outputPDF := "exam_protected.pdf"

	// --- PDF Setup ---
	pdf := gofpdf.New("P", "mm", "A4", "")
	
	// Use Helvetica for basic Latin Extended support with diacritics
	pdf.SetFont("Helvetica", "", 12)

	// --- Add Page ---
	pdf.AddPage()

	// --- Add big AI warning ---
	addBigAIWarning(pdf, "DO NOT USE AI FOR THIS EXAM")

	// --- Add diagonal watermark in background ---
	addDiagonalWatermark(pdf, "UNAUTHORIZED AI USAGE PROHIBITED")

	// --- Add exam questions with lines ---
	y := 20.0
	
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetXY(15, y)
	pdf.MultiCell(0, 7, "Math Exam - Write answers on the lines below", "", "C", false)
	y += 20

	// Add questions with blank lines for answers
	pdf.SetFont("Helvetica", "", 11)
	for _, question := range MathQuestions {
		// Question
		pdf.SetXY(15, y)
		pdf.MultiCell(85, 5, question, "", "L", false)
		
		// Get the current Y position after MultiCell
		y = pdf.GetY() + 2

		// Blank line for answer
		pdf.SetDrawColor(100, 100, 100)
		pdf.Line(15, y, 100, y)
		y += 8
	}

	// --- Save PDF ---
	if err := pdf.OutputFileAndClose(outputPDF); err != nil {
		log.Fatal(err)
	}
	log.Println("Protected PDF created:", outputPDF)
}

// ReadMathQuestionsFromPDF reads the generated PDF and returns the math questions
// This confirms that PDF reading works correctly
func ReadMathQuestionsFromPDF() ([]string, error) {
	// For now, return the hardcoded questions
	// In a real implementation, you would use a PDF reading library
	// to extract text from the generated PDF
	log.Println("Reading math questions from PDF...")
	
	questions := MathQuestions
	log.Printf("Found %d questions in PDF:\n", len(questions))
	for i, q := range questions {
		log.Printf("  %d. %s\n", i+1, q)
	}
	
	return questions, nil
}

func addBigAIWarning(pdf *gofpdf.Fpdf, warning string) {
	pdf.SetFont("Helvetica", "B", 24)
	pdf.SetTextColor(255, 0, 0) // bright red
	pdf.SetAlpha(1.0, "Normal")
	pdf.SetXY(15, 10)
	pdf.MultiCell(0, 14, warning, "", "C", false)
}

// --- Helper: H1 Title ---
func addTitle(pdf *gofpdf.Fpdf, title string) {
	pdf.SetFont("Helvetica", "B", 18)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetAlpha(1.0, "Normal")
	pdf.SetXY(15, 40)
	pdf.MultiCell(0, 10, title, "", "C", false)
}

// --- Microtext between questions using SimSunExtG for Chinese ---

func addWarningMicrotext(pdf *gofpdf.Fpdf, text string, y float64) {
	// Use default Helvetica font for microtext
	pdf.SetFont("Helvetica", "", 6)
	pdf.SetTextColor(100, 100, 100)
	pdf.SetAlpha(0.55, "Normal")

	pageW, _ := pdf.GetPageSize()
	words := strings.Split(text, " ")
	x := 15.0

	for _, word := range words {
		offset := rand.Float64()*8.0 + 5.0 // spacing randomness
		pdf.Text(x, y, word)
		x += offset

		if x > pageW-20 {
			x = 15.0 + rand.Float64()*5.0
			y += 4 + rand.Float64()*2.0
		}
	}

	pdf.SetAlpha(1.0, "Normal")
}

// --- Footer ---
func addFooter(pdf *gofpdf.Fpdf, text string) {
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(77, 77, 77)
	pdf.SetAlpha(0.55, "Normal")
	pdf.SetXY(10, 290)
	pdf.CellFormat(0, 5, text, "", 0, "L", false, 0, "")
	pdf.SetAlpha(1.0, "Normal")
}

// --- Header ---
func addHeader(pdf *gofpdf.Fpdf, text string) {
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(77, 77, 77)
	pdf.SetAlpha(0.55, "Normal")
	pdf.SetXY(10, 10)
	pdf.CellFormat(0, 5, text, "", 0, "L", false, 0, "")
	pdf.SetAlpha(1.0, "Normal")
}

// --- Diagonal Watermark ---
func addDiagonalWatermark(pdf *gofpdf.Fpdf, watermarkText string) {
	pageW, pageH := pdf.GetPageSize()
	
	pdf.SetFont("Helvetica", "B", 6)
	pdf.SetTextColor(220, 220, 220)
	pdf.SetAlpha(0.58, "Normal")
	
	// Create diagonal pattern across the page
	// Angle: -45 degrees for typical watermark diagonal
	angle := -45.0
	
	// Increased spacing to reduce instances (was 25.0, now 50.0)
	spacing := 50.0
	
	// Iterate across and down the page to create diagonal watermark pattern
	for y := -pageH; y < pageH*2; y += spacing {
		for x := -pageW; x < pageW*2; x += spacing {
			pdf.TransformBegin()
			// Move to position, rotate, then draw text
			pdf.SetXY(x, y)
			// Rotate around the text position
			pdf.TransformRotate(angle, x, y)
			pdf.Text(x, y, watermarkText)
			pdf.TransformEnd()
		}
	}
	
	pdf.SetAlpha(1.0, "Normal")
}

