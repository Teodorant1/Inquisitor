package printer

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

// GenerateDynamicProtectedPDF processes customized payloads dynamically
func GenerateDynamicProtectedPDF(outputPDF string, questions []string, warning string, watermark string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	
	// Ensure your project directory has access to the .ttf file
	pdf.AddUTF8Font("NotoSans", "", "NotoSans-Regular.ttf")
	pdf.SetFont("NotoSans", "", 10)

	// Inject dynamic parameters securely
	if watermark != "" {
		addDiagonalWatermark(pdf, watermark)
	}
	if warning != "" {
		addBigAIWarning(pdf, warning)
	}

	y := 30.0
	pdf.SetTextColor(0, 0, 0)
	
	for _, question := range questions {
		pdf.SetXY(15, y)
		pdf.MultiCell(180, 5, question, "", "L", false)
		
		y = pdf.GetY() + 2
		pdf.SetDrawColor(100, 100, 100)
		pdf.Line(15, y, 100, y)
		y += 10
	}

	return pdf.OutputFileAndClose(outputPDF)
}

// ExtractUploadedPDF converts an uploaded file via pdftotext system binaries
func ExtractUploadedPDF(filename string) ([]string, error) {
	cmd := exec.Command("pdftotext", "-enc", "UTF-8", filename, "-")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pdftotext execution failed: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	var cleaned []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			cleaned = append(cleaned, strings.TrimSpace(line))
		}
	}
	return cleaned, nil
}

// Internal formatting visual components remain uncoupled 
func addBigAIWarning(pdf *gofpdf.Fpdf, warning string) {
	pdf.SetTextColor(255, 0, 0)
	pdf.SetXY(15, 10)
	pdf.MultiCell(0, 14, warning, "", "C", false)
}

func addDiagonalWatermark(pdf *gofpdf.Fpdf, watermarkText string) {
	pageW, pageH := pdf.GetPageSize()
	pdf.SetTextColor(220, 220, 220)
	pdf.SetAlpha(0.58, "Normal")
	
	angle := -45.0
	spacing := 50.0
	
	for y := -pageH; y < pageH*2; y += spacing {
		for x := -pageW; x < pageW*2; x += spacing {
			pdf.TransformBegin()
			pdf.SetXY(x, y)
			pdf.TransformRotate(angle, x, y)
			pdf.Text(x, y, watermarkText)
			pdf.TransformEnd()
		}
	}
	pdf.SetAlpha(1.0, "Normal")
}