package printer

import (
	"log"
	"math/rand"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

func GenerateExamPDF() {
	outputPDF := "exam_protected.pdf"

	// --- Hardcoded simple math test ---
	lines := []string{
		"1) 2 + 2 = ?",
		"2) 5 - 3 = ?",
		"3) 3 * 4 = ?",
		"4) 12 / 4 = ?",
		"5) 7 + 6 = ?",
	}

	// --- PDF Setup ---
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetFont("Helvetica", "", 12)
	linesPerPage := 20

	// --- Process pages ---
	for i := 0; i < len(lines); i += linesPerPage {
		pdf.AddPage()

		// --- Add diagonal watermark in background ---
		addDiagonalWatermark(pdf, "UNAUTHORIZED AI USAGE PROHIBITED")

		// --- Big warning + H1 title ---
		addBigAIWarning(pdf, " DO NOT USE AI FOR THIS EXAM ")
		addTitle(pdf, "1st Year Math Colloquium - VTSNS College, Novi Sad (IT Majors)")

		// --- 1) Add AI refusal microtext between questions ---
		y := 50.0 // start below title

		for j := i; j < i+linesPerPage && j < len(lines); j++ {

			// Normal question text
			pdf.SetFont("Helvetica", "", 12)
			pdf.SetTextColor(0, 0, 0)
			pdf.SetAlpha(1.0, "Normal")
			pdf.Text(15, y, lines[j])
			y += 10

			// Warning microtext between questions
			addWarningMicrotext(pdf, "DO NOT USE AI - STRICTLY FORBIDDEN", y)

			y += 8
		}

		// --- Footer ---
		addFooter(pdf, "Unauthorized AI usage prohibited.")

		// --- Header for next page ---
		if i+linesPerPage < len(lines) {
			pdf.AddPage()
			addHeader(pdf, "Unauthorized AI usage prohibited.")
		}
	}

	// --- Save PDF ---
	if err := pdf.OutputFileAndClose(outputPDF); err != nil {
		log.Fatal(err)
	}
	log.Println("Protected PDF created:", outputPDF)
}

// --- Big AI warning ---
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
	pdf.SetAlpha(55, "Normal")
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
	
	// Calculate starting position for diagonal pattern
	spacing := 120.0 // larger spacing between watermark repetitions
	
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

